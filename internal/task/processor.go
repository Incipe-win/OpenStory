package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/Incipe-win/OpenStory/internal/asset"
	"github.com/Incipe-win/OpenStory/internal/billing"
	"github.com/Incipe-win/OpenStory/internal/compose"
	"github.com/Incipe-win/OpenStory/internal/eventbus"
	"github.com/Incipe-win/OpenStory/internal/observability"
	"github.com/Incipe-win/OpenStory/internal/provider"
)

// Payload is the Asynq task payload for generation tasks.
type Payload struct {
	TaskID      uuid.UUID `json:"task_id"`
	RequestID   string    `json:"request_id,omitempty"`
	TraceID     string    `json:"trace_id,omitempty"`
	TraceParent string    `json:"traceparent,omitempty"`
}

// NewAsynqTask creates a new Asynq task for a generation task.
func NewAsynqTask(taskID uuid.UUID) (*asynq.Task, error) {
	return NewAsynqTaskWithContext(context.Background(), taskID)
}

func NewAsynqTaskWithContext(ctx context.Context, taskID uuid.UUID) (*asynq.Task, error) {
	payload, err := json.Marshal(Payload{
		TaskID:      taskID,
		RequestID:   observability.RequestIDFromContext(ctx),
		TraceID:     observability.TraceIDFromContext(ctx),
		TraceParent: observability.TraceParentFromContext(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return asynq.NewTask(AsynqTaskType, payload,
		asynq.MaxRetry(3),
		asynq.Timeout(5*time.Minute),
		asynq.Queue("generation"),
		asynq.TaskID(taskID.String()),
	), nil
}

// Processor handles Asynq generation tasks.
type Processor struct {
	repo        Repository
	registry    *provider.Registry
	outbox      eventbus.EventBus
	billingSvc  billing.Service
	recorder    provider.CallRecorder
	composer    *compose.Service
	maxAttempts int

	assetCreator *asset.Creator
	storage      *asset.Storage
	asynqClient  *asynq.Client

	log zerolog.Logger
}

// NewProcessor creates a new task processor.
func NewProcessor(repo Repository, registry *provider.Registry, outbox eventbus.EventBus, billingSvc billing.Service, recorder provider.CallRecorder, maxAttempts int, log zerolog.Logger) *Processor {
	if maxAttempts <= 0 {
		maxAttempts = 2
	}
	return &Processor{repo: repo, registry: registry, outbox: outbox, billingSvc: billingSvc, recorder: recorder, maxAttempts: maxAttempts, log: log}
}

func (p *Processor) SetComposer(composer *compose.Service) {
	p.composer = composer
}

func (p *Processor) SetAssetCreator(creator *asset.Creator, storage *asset.Storage) {
	p.assetCreator = creator
	p.storage = storage
}

func (p *Processor) SetAsynqClient(client *asynq.Client) {
	p.asynqClient = client
}

// ProcessTask is called by Asynq when a generation task is dequeued.
func (p *Processor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	started := time.Now()
	var payload Payload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	ctx = observability.ContextWithTraceParentHeader(ctx, payload.TraceParent)
	ctx = observability.ContextWithIDs(ctx, payload.RequestID, payload.TraceID)
	ctx, span := otel.Tracer("openstory/worker").Start(ctx, "generation.execute")
	defer span.End()
	ctx = observability.ContextWithTraceID(ctx, observability.TraceIDFromContext(ctx))
	ctx = observability.ContextWithTraceParent(ctx, observability.TraceParentFromContext(ctx))
	span.SetAttributes(attribute.String("task.id", payload.TaskID.String()))

	log := p.log.With().
		Str("task_id", payload.TaskID.String()).
		Str("request_id", observability.RequestIDFromContext(ctx)).
		Str("trace_id", observability.TraceIDFromContext(ctx)).
		Logger()
	log.Info().Msg("processing generation task")

	// Get task from DB
	task, err := p.repo.GetByID(ctx, payload.TaskID)
	if err != nil {
		log.Error().Err(err).Msg("task not found")
		observability.ObserveTask("unknown", "unknown", StatusFailed, time.Since(started))
		return fmt.Errorf("get task: %w", err)
	}
	span.SetAttributes(
		attribute.String("task.type", task.Type),
		attribute.String("task.provider", task.Provider),
		attribute.String("user.id", task.UserID.String()),
	)

	// Skip if already terminal
	if IsTerminal(task.Status) {
		log.Info().Str("status", task.Status).Msg("task already terminal, skipping")
		return nil
	}

	reservedCredits := EstimateCostCredits(task.Type)
	if reservedCredits > 0 && p.billingSvc != nil {
		if _, err := p.billingSvc.Reserve(ctx, task.UserID, reservedCredits, "generation_task", task.ID, "Reserve credits for generation task"); err != nil {
			if errors.Is(err, billing.ErrInsufficientCredits) || errors.Is(err, billing.ErrAccountNotFound) {
				p.failTask(ctx, task, err.Error(), log)
				observability.ObserveTask(task.Type, task.Provider, StatusFailed, time.Since(started))
				return nil
			}
			log.Error().Err(err).Msg("failed to reserve credits")
			return fmt.Errorf("reserve credits: %w", err)
		}
	}

	// Set running
	if err := p.repo.SetRunning(ctx, task.ID); err != nil {
		log.Error().Err(err).Msg("failed to set running")
		return fmt.Errorf("set running: %w", err)
	}
	_ = p.repo.AddEvent(ctx, task.ID, EventStarted, map[string]any{
		"retry_count": task.RetryCount,
	})
	p.publishTaskEvent(ctx, "task_started", task, map[string]any{
		"status":      StatusRunning,
		"retry_count": task.RetryCount,
	})

	// Get provider
	providerName := task.Provider
	if providerName == "" {
		providerName = "openai-compatible"
		task.Provider = providerName
	}
	result, _, err := p.execute(ctx, task, providerName)
	if err != nil {
		// Check if context was canceled (task cancel or timeout)
		if ctx.Err() != nil {
			_ = p.repo.Cancel(ctx, task.ID)
			_ = p.repo.AddEvent(context.Background(), task.ID, EventCanceled, map[string]any{
				"reason": ctx.Err().Error(),
			})
			p.refundReservedCredits(context.Background(), task, "Task canceled")
			p.publishTaskEvent(context.Background(), "task_canceled", task, map[string]any{
				"status": StatusCanceled,
				"reason": ctx.Err().Error(),
			})
			observability.ObserveTask(task.Type, task.Provider, StatusCanceled, time.Since(started))
			log.Info().Msg("task canceled via context")
			return nil
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "not registered") || strings.Contains(errMsg, "not configured") {
			p.refundReservedCredits(ctx, task, "Task failed before provider execution")
			p.failTask(ctx, task, errMsg, log)
			observability.ObserveTask(task.Type, task.Provider, StatusFailed, time.Since(started))
			return nil
		}
		if isFinalAsynqAttempt(ctx) {
			p.refundReservedCredits(ctx, task, "Task failed after retries")
			p.failTask(ctx, task, errMsg, log)
			observability.ObserveTask(task.Type, task.Provider, StatusFailed, time.Since(started))
			return nil
		}
		// Increment retry count
		_ = p.repo.IncrRetry(ctx, task.ID)
		_ = p.repo.AddEvent(ctx, task.ID, EventRetried, map[string]any{
			"error": errMsg,
		})
		p.publishTaskEvent(ctx, "task_retried", task, map[string]any{
			"status": StatusRunning,
			"error":  errMsg,
		})
		log.Warn().Err(err).Msg("provider returned error, will retry")
		return err // Asynq will retry
	}

	// Success - update task
	outputJSON := result.Output
	if p.billingSvc != nil {
		if _, err := p.billingSvc.Confirm(ctx, task.UserID, result.CostCredits, "generation_task", task.ID, "Confirm generation task credits"); err != nil {
			log.Error().Err(err).Msg("failed to confirm credits")
			return fmt.Errorf("confirm credits: %w", err)
		}
	}
	if err := p.repo.UpdateStatus(ctx, task.ID, StatusSucceeded, &outputJSON, nil); err != nil {
		log.Error().Err(err).Msg("failed to update succeeded status")
		return fmt.Errorf("update status: %w", err)
	}

	// Create assets from result (best-effort, errors are logged but don't fail the task)
	p.createAssetsFromResult(ctx, task, &outputJSON)

	_ = p.repo.AddEvent(ctx, task.ID, EventCompleted, map[string]any{
		"cost_credits": result.CostCredits,
	})
	p.publishTaskEvent(ctx, "task_succeeded", task, map[string]any{
		"status":       StatusSucceeded,
		"cost_credits": result.CostCredits,
	})

	_ = p.repo.UpdateCost(ctx, task.ID, result.CostCredits)

	log.Info().
		Int("cost", result.CostCredits).
		Msg("task completed successfully")
	observability.ObserveTask(task.Type, task.Provider, StatusSucceeded, time.Since(started))

	return nil
}

func (p *Processor) execute(ctx context.Context, task *GenerationTask, providerName string) (*provider.Result, time.Duration, error) {
	if task.Type == TypeCompose && providerName == "ffmpeg" {
		if p.composer == nil {
			err := fmt.Errorf("ffmpeg composer is not configured")
			p.recordProviderCall(ctx, task.ID, providerName, provider.CapabilityVideoGenerate, "ffmpeg", "failed", 0, provider.Usage{}, 0, err)
			return nil, 0, err
		}
		started := time.Now()
		result, err := p.composer.Compose(ctx, task.ID, task.UserID, task.ProjectID, task.InputJSON)
		duration := time.Since(started)
		if err != nil {
			p.recordProviderCall(ctx, task.ID, providerName, provider.CapabilityVideoGenerate, "ffmpeg", "failed", duration, provider.Usage{}, 0, err)
			return nil, duration, err
		}
		output, _ := json.Marshal(result)
		p.recordProviderCall(ctx, task.ID, providerName, provider.CapabilityVideoGenerate, "ffmpeg", "succeeded", duration, provider.Usage{}, 0, nil)
		return &provider.Result{
			Output:     output,
			Capability: provider.CapabilityVideoGenerate,
			Model:      "ffmpeg",
		}, duration, nil
	}

	prov, ok := p.registry.Get(providerName)
	if !ok {
		err := fmt.Errorf("provider %q not registered", providerName)
		p.recordProviderCall(ctx, task.ID, providerName, provider.CapabilityForTask(task.Type), "", "failed", 0, provider.Usage{}, 0, err)
		return nil, 0, err
	}

	req := provider.Request{
		TaskType: task.Type,
		Provider: providerName,
		Input:    task.InputJSON,
	}

	started := time.Now()
	result, err := provider.Execute(ctx, prov, req, p.maxAttempts)
	duration := time.Since(started)
	if err != nil {
		p.recordProviderCall(ctx, task.ID, providerName, provider.CapabilityForTask(task.Type), "", "failed", duration, provider.Usage{}, 0, err)
		return nil, duration, err
	}
	p.recordProviderCall(ctx, task.ID, providerName, result.Capability, result.Model, "succeeded", duration, result.Usage, result.CostCredits, nil)
	return result, duration, nil
}

func (p *Processor) recordProviderCall(ctx context.Context, taskID uuid.UUID, providerName string, capability provider.Capability, model, status string, duration time.Duration, usage provider.Usage, costCredits int, callErr error) {
	observability.ObserveProviderCall(providerName, string(capability), status, duration)
	if p.recorder == nil {
		return
	}
	var errMsg *string
	if callErr != nil {
		msg := callErr.Error()
		errMsg = &msg
	}
	if err := p.recorder.Record(ctx, provider.CallLog{
		TaskID:       taskID,
		Provider:     providerName,
		Capability:   capability,
		Model:        model,
		Status:       status,
		Duration:     duration,
		Usage:        usage,
		CostCredits:  costCredits,
		ErrorMessage: errMsg,
	}); err != nil {
		p.log.Warn().Err(err).Str("provider", providerName).Msg("failed to record provider call")
	}
}

func (p *Processor) failTask(ctx context.Context, task *GenerationTask, errMsg string, log zerolog.Logger) {
	_ = p.repo.UpdateStatus(ctx, task.ID, StatusFailed, nil, &errMsg)
	_ = p.repo.AddEvent(ctx, task.ID, EventFailed, map[string]any{"error": errMsg})
	p.publishTaskEvent(ctx, "task_failed", task, map[string]any{
		"status": StatusFailed,
		"error":  errMsg,
	})
	log.Error().Str("error", errMsg).Msg("task failed permanently")
}

func (p *Processor) refundReservedCredits(ctx context.Context, task *GenerationTask, reason string) {
	if p.billingSvc == nil {
		return
	}
	if _, err := p.billingSvc.Refund(ctx, task.UserID, "generation_task", task.ID, reason); err != nil &&
		!errors.Is(err, billing.ErrReservationNotFound) &&
		!errors.Is(err, billing.ErrReservationConfirmed) {
		p.log.Warn().Err(err).Str("task_id", task.ID.String()).Msg("failed to refund reserved credits")
	}
}

func isFinalAsynqAttempt(ctx context.Context) bool {
	retryCount, okRetry := asynq.GetRetryCount(ctx)
	maxRetry, okMax := asynq.GetMaxRetry(ctx)
	return okRetry && okMax && retryCount >= maxRetry
}

func (p *Processor) publishTaskEvent(ctx context.Context, eventType string, task *GenerationTask, payload map[string]any) {
	if p.outbox == nil {
		return
	}
	payload["task_type"] = task.Type
	payload["provider"] = task.Provider
	payload["project_id"] = task.ProjectID
	_ = p.outbox.Publish(ctx, eventbus.TopicGenerationTaskEvents,
		eventbus.NewEvent(eventType, "generation_task", task.ID, payload).WithUser(task.UserID))
}

// ── Asset Creation ───────────────────────────────────

func (p *Processor) createAssetsFromResult(ctx context.Context, task *GenerationTask, output *json.RawMessage) {
	if output == nil || p.assetCreator == nil || p.storage == nil {
		return
	}

	switch task.Type {
	case TypeImageGeneration:
		p.createImageAssets(ctx, task, *output)
	case TypeVideoGeneration:
		p.createVideoAssets(ctx, task, *output)
	case TypeCreativePipeline:
		p.cascadeCreativePipeline(ctx, task, *output)
	}
}

func (p *Processor) createImageAssets(ctx context.Context, task *GenerationTask, output json.RawMessage) {
	var res struct {
		Images []struct {
			URL           string `json:"url"`
			Width         int    `json:"width"`
			Height        int    `json:"height"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"images"`
	}
	if err := json.Unmarshal(output, &res); err != nil {
		p.log.Warn().Err(err).Str("task_id", task.ID.String()).Msg("failed to parse image gen output for asset creation")
		return
	}

	for i, img := range res.Images {
		if img.URL == "" {
			continue
		}
		name := fmt.Sprintf("generated-image-%03d", i+1)
		if err := p.downloadAndCreateAsset(ctx, task, name, "image/png", img.URL, img.RevisedPrompt, img.Width, img.Height, 0); err != nil {
			p.log.Warn().Err(err).Str("task_id", task.ID.String()).Int("image_index", i).Msg("failed to create image asset")
		}
	}
}

func (p *Processor) createVideoAssets(ctx context.Context, task *GenerationTask, output json.RawMessage) {
	var res struct {
		Videos []struct {
			URL             string `json:"url"`
			DurationSeconds int    `json:"duration_seconds"`
			Width           int    `json:"width"`
			Height          int    `json:"height"`
		} `json:"videos"`
	}
	if err := json.Unmarshal(output, &res); err != nil {
		p.log.Warn().Err(err).Str("task_id", task.ID.String()).Msg("failed to parse video gen output for asset creation")
		return
	}

	for i, vid := range res.Videos {
		if vid.URL == "" {
			continue
		}
		name := fmt.Sprintf("generated-video-%03d", i+1)
		durationMs := vid.DurationSeconds * 1000
		if err := p.downloadAndCreateAsset(ctx, task, name, "video/mp4", vid.URL, "", vid.Width, vid.Height, durationMs); err != nil {
			p.log.Warn().Err(err).Str("task_id", task.ID.String()).Int("video_index", i).Msg("failed to create video asset")
		}
	}
}

func (p *Processor) downloadAndCreateAsset(ctx context.Context, task *GenerationTask, name, mimeType, sourceURL, extraJSON string, width, height, durationMs int) error {
	tmpDir, err := os.MkdirTemp("", "openstory-asset-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	ext := filepath.Ext(name)
	if ext == "" {
		ext = extensionForMime(mimeType)
		name += ext
	}
	tmpFile := filepath.Join(tmpDir, name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", sourceURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("download returned %d for %s", resp.StatusCode, sourceURL)
	}

	f, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 256<<20)); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	f.Close()

	assetID := uuid.New()
	storageKey := asset.BuildStorageKey(task.ProjectID.String(), assetID.String(), name)
	if _, err := p.storage.FPutObject(ctx, storageKey, tmpFile, mimeType); err != nil {
		return fmt.Errorf("upload to storage: %w", err)
	}

	size, _ := asset.FileSize(tmpFile)
	checksum, _ := asset.ChecksumSHA256(tmpFile)

	meta := json.RawMessage("{}")
	if extraJSON != "" {
		meta, _ = json.Marshal(map[string]string{"prompt": extraJSON})
	}

	a := &asset.Asset{
		ID:            assetID,
		UserID:        task.UserID,
		ProjectID:     &task.ProjectID,
		Type:          assetTypeFromMime(mimeType),
		Name:          name,
		MimeType:      mimeType,
		SizeBytes:     size,
		DurationMs:    intPtr(durationMs),
		Width:         intPtr(width),
		Height:        intPtr(height),
		Checksum:      checksum,
		StorageKey:    storageKey,
		StorageBucket: p.storage.Bucket(),
		URL:           asset.ObjectURL(p.storage.Bucket(), storageKey),
		MetadataJSON:  meta,
	}
	if err := p.assetCreator.Create(ctx, a); err != nil {
		return fmt.Errorf("create asset record: %w", err)
	}

	p.log.Info().
		Str("asset_id", assetID.String()).
		Str("task_id", task.ID.String()).
		Str("type", a.Type).
		Msg("asset created from generation task")
	return nil
}

// ── Cascade: creative_pipeline → child media tasks ────

func (p *Processor) cascadeCreativePipeline(ctx context.Context, task *GenerationTask, output json.RawMessage) {
	if p.asynqClient == nil {
		p.log.Warn().Str("task_id", task.ID.String()).Msg("no asynq client available for cascade")
		return
	}

	var pipelineOutput struct {
		ImagePrompts json.RawMessage `json:"image_prompts"`
		VideoPrompts json.RawMessage `json:"video_prompts"`
		Storyboard   json.RawMessage `json:"storyboard"`
		Script       json.RawMessage `json:"script"`
		Characters   json.RawMessage `json:"characters"`
	}
	if err := json.Unmarshal(output, &pipelineOutput); err != nil {
		p.log.Warn().Err(err).Str("task_id", task.ID.String()).Msg("failed to parse pipeline output for cascade")
		return
	}

	if len(pipelineOutput.ImagePrompts) > 0 {
		var imgPrompts struct {
			Prompts []struct {
				ShotIndex      int    `json:"shot_index"`
				Prompt         string `json:"prompt"`
				NegativePrompt string `json:"negative_prompt"`
			} `json:"prompts"`
		}
		if err := json.Unmarshal(pipelineOutput.ImagePrompts, &imgPrompts); err == nil {
			for _, ip := range imgPrompts.Prompts {
				p.createChildTask(ctx, task, TypeImageGeneration, "openai-compatible", map[string]any{
					"prompt":          ip.Prompt,
					"negative_prompt": ip.NegativePrompt,
					"shot_index":      ip.ShotIndex,
					"source":          "cascade",
				})
			}
		}
	}

	if len(pipelineOutput.VideoPrompts) > 0 {
		var vidPrompts struct {
			Prompts []struct {
				ShotIndex      int    `json:"shot_index"`
				Prompt         string `json:"prompt"`
				DurationSeconds int   `json:"duration_seconds"`
				CameraFixed    bool   `json:"camera_fixed"`
			} `json:"prompts"`
		}
		if err := json.Unmarshal(pipelineOutput.VideoPrompts, &vidPrompts); err == nil {
			for _, vp := range vidPrompts.Prompts {
				p.createChildTask(ctx, task, TypeVideoGeneration, "openai-compatible", map[string]any{
					"prompt":           vp.Prompt,
					"shot_index":       vp.ShotIndex,
					"duration_seconds": vp.DurationSeconds,
					"camera_fixed":     vp.CameraFixed,
					"source":           "cascade",
				})
			}
		}
	}

	// Create text asset for the pipeline output itself
	p.createTextOutputAsset(ctx, task, output)
}

func (p *Processor) createChildTask(ctx context.Context, parentTask *GenerationTask, taskType, provider string, input map[string]any) {
	inputJSON, _ := json.Marshal(input)

	now := time.Now()
	childTask := &GenerationTask{
		UserID:         parentTask.UserID,
		ProjectID:      parentTask.ProjectID,
		WorkflowID:     parentTask.WorkflowID,
		NodeID:         parentTask.NodeID,
		Type:           taskType,
		Provider:       provider,
		Status:         StatusPending,
		IdempotencyKey: fmt.Sprintf("cascade:%s:%s:%d", parentTask.ID, taskType, now.UnixNano()),
		InputJSON:      inputJSON,
	}

	if err := p.repo.Create(ctx, childTask); err != nil {
		p.log.Warn().Err(err).Str("parent_task_id", parentTask.ID.String()).Str("child_type", taskType).Msg("failed to create child task")
		return
	}

	_ = p.repo.AddEvent(ctx, childTask.ID, EventCreated, map[string]any{
		"type":        taskType,
		"provider":    provider,
		"parent_task": parentTask.ID,
		"workflow_id": parentTask.WorkflowID,
	})

	asynqTask, err := NewAsynqTaskWithContext(ctx, childTask.ID)
	if err != nil {
		p.log.Warn().Err(err).Str("child_task_id", childTask.ID.String()).Msg("failed to create child asynq task")
		return
	}

	if _, err := p.asynqClient.Enqueue(asynqTask); err != nil {
		p.log.Warn().Err(err).Str("child_task_id", childTask.ID.String()).Msg("failed to enqueue child task")
		return
	}

	_ = p.repo.UpdateStatus(ctx, childTask.ID, StatusQueued, nil, nil)
	_ = p.repo.AddEvent(ctx, childTask.ID, EventQueued, map[string]any{
		"parent_task": parentTask.ID,
	})

	p.log.Info().
		Str("child_task_id", childTask.ID.String()).
		Str("parent_task_id", parentTask.ID.String()).
		Str("type", taskType).
		Msg("cascaded child task created")
}

func (p *Processor) createTextOutputAsset(ctx context.Context, task *GenerationTask, output json.RawMessage) {
	outputStr := string(output)
	tmpDir, err := os.MkdirTemp("", "openstory-text-*")
	if err != nil {
		p.log.Warn().Err(err).Str("task_id", task.ID.String()).Msg("failed to create text asset temp dir")
		return
	}
	defer os.RemoveAll(tmpDir)

	name := "pipeline-output.json"
	tmpFile := filepath.Join(tmpDir, name)
	if err := os.WriteFile(tmpFile, []byte(outputStr), 0o600); err != nil {
		p.log.Warn().Err(err).Str("task_id", task.ID.String()).Msg("failed to write text asset temp file")
		return
	}

	assetID := uuid.New()
	mimeType := "application/json"
	storageKey := asset.BuildStorageKey(task.ProjectID.String(), assetID.String(), name)
	if _, err := p.storage.FPutObject(ctx, storageKey, tmpFile, mimeType); err != nil {
		p.log.Warn().Err(err).Str("task_id", task.ID.String()).Msg("failed to upload text asset")
		return
	}

	a := &asset.Asset{
		ID:            assetID,
		UserID:        task.UserID,
		ProjectID:     &task.ProjectID,
		Type:          "text",
		Name:          name,
		MimeType:      mimeType,
		SizeBytes:     int64(len(outputStr)),
		Checksum:      "",
		StorageKey:    storageKey,
		StorageBucket: p.storage.Bucket(),
		URL:           asset.ObjectURL(p.storage.Bucket(), storageKey),
		MetadataJSON:  json.RawMessage(`{"task_type":"creative_pipeline"}`),
	}
	if err := p.assetCreator.Create(ctx, a); err != nil {
		p.log.Warn().Err(err).Str("task_id", task.ID.String()).Msg("failed to create text asset record")
		return
	}
	p.log.Info().Str("asset_id", assetID.String()).Msg("text output asset created for creative_pipeline")
}

// ── Helpers ──────────────────────────────────────────

func assetTypeFromMime(mime string) string {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	default:
		return "file"
	}
}

func extensionForMime(mime string) string {
	switch strings.ToLower(mime) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav":
		return ".wav"
	default:
		return ".bin"
	}
}

func intPtr(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}
