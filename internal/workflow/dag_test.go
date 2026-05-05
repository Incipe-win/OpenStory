package workflow

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeNode(id uuid.UUID, t NodeType) Node {
	return Node{ID: id, Type: t, Name: string(t)}
}

func makeEdge(src, tgt uuid.UUID) Edge {
	return Edge{SourceNodeID: src, TargetNodeID: tgt, SourceHandle: "default", TargetHandle: "default"}
}

// ─── ValidateDAG Tests ──────────────────────────────

func TestValidateDAG_Empty(t *testing.T) {
	err := ValidateDAG(nil, nil)
	assert.ErrorIs(t, err, ErrEmptyWorkflow)
}

func TestValidateDAG_SingleNode(t *testing.T) {
	n := makeNode(uuid.New(), NodeIdea)
	err := ValidateDAG([]Node{n}, nil)
	assert.NoError(t, err)
}

func TestValidateDAG_LinearChain(t *testing.T) {
	// idea → script → scene
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	nodes := []Node{
		makeNode(a, NodeIdea),
		makeNode(b, NodeScript),
		makeNode(c, NodeScene),
	}
	edges := []Edge{makeEdge(a, b), makeEdge(b, c)}
	assert.NoError(t, ValidateDAG(nodes, edges))
}

func TestValidateDAG_Diamond(t *testing.T) {
	// idea → script → character ─┐
	//                  └─ scene ──┘→ storyboard
	a, b, c, d, e := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	nodes := []Node{
		makeNode(a, NodeIdea),
		makeNode(b, NodeScript),
		makeNode(c, NodeCharacter),
		makeNode(d, NodeScene),
		makeNode(e, NodeStoryboard),
	}
	edges := []Edge{
		makeEdge(a, b),
		makeEdge(b, c), makeEdge(b, d),
		makeEdge(c, e), makeEdge(d, e),
	}
	assert.NoError(t, ValidateDAG(nodes, edges))
}

func TestValidateDAG_Cycle(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	nodes := []Node{
		makeNode(a, NodeIdea),
		makeNode(b, NodeScript),
		makeNode(c, NodeScene),
	}
	edges := []Edge{makeEdge(a, b), makeEdge(b, c), makeEdge(c, a)}
	err := ValidateDAG(nodes, edges)
	assert.ErrorIs(t, err, ErrCycleDetected)
}

func TestValidateDAG_SelfLoop(t *testing.T) {
	a := uuid.New()
	nodes := []Node{makeNode(a, NodeIdea)}
	edges := []Edge{makeEdge(a, a)}
	err := ValidateDAG(nodes, edges)
	assert.ErrorIs(t, err, ErrSelfLoop)
}

func TestValidateDAG_OrphanEdge(t *testing.T) {
	a := uuid.New()
	nodes := []Node{makeNode(a, NodeIdea)}
	edges := []Edge{makeEdge(a, uuid.New())} // target doesn't exist
	err := ValidateDAG(nodes, edges)
	assert.ErrorIs(t, err, ErrOrphanNode)
}

func TestValidateDAG_InvalidNodeType(t *testing.T) {
	a := uuid.New()
	nodes := []Node{{ID: a, Type: "invalid_type", Name: "bad"}}
	err := ValidateDAG(nodes, nil)
	assert.ErrorIs(t, err, ErrInvalidNodeType)
}

func TestValidateDAG_TwoNodeCycle(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	nodes := []Node{makeNode(a, NodeIdea), makeNode(b, NodeScript)}
	edges := []Edge{makeEdge(a, b), makeEdge(b, a)}
	assert.ErrorIs(t, ValidateDAG(nodes, edges), ErrCycleDetected)
}

// ─── TopologicalSort Tests ──────────────────────────

func TestTopologicalSort_Empty(t *testing.T) {
	sorted, err := TopologicalSort(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, sorted)
}

func TestTopologicalSort_SingleNode(t *testing.T) {
	n := makeNode(uuid.New(), NodeIdea)
	sorted, err := TopologicalSort([]Node{n}, nil)
	require.NoError(t, err)
	assert.Len(t, sorted, 1)
	assert.Equal(t, n.ID, sorted[0].ID)
}

func TestTopologicalSort_LinearChain(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	nodes := []Node{
		makeNode(a, NodeIdea),
		makeNode(b, NodeScript),
		makeNode(c, NodeScene),
	}
	edges := []Edge{makeEdge(a, b), makeEdge(b, c)}

	sorted, err := TopologicalSort(nodes, edges)
	require.NoError(t, err)
	require.Len(t, sorted, 3)

	// a must come before b, b before c
	idx := make(map[uuid.UUID]int)
	for i, n := range sorted {
		idx[n.ID] = i
	}
	assert.Less(t, idx[a], idx[b])
	assert.Less(t, idx[b], idx[c])
}

func TestTopologicalSort_Diamond(t *testing.T) {
	a, b, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	nodes := []Node{
		makeNode(a, NodeIdea),
		makeNode(b, NodeCharacter),
		makeNode(c, NodeScene),
		makeNode(d, NodeStoryboard),
	}
	edges := []Edge{
		makeEdge(a, b), makeEdge(a, c),
		makeEdge(b, d), makeEdge(c, d),
	}

	sorted, err := TopologicalSort(nodes, edges)
	require.NoError(t, err)
	require.Len(t, sorted, 4)

	idx := make(map[uuid.UUID]int)
	for i, n := range sorted {
		idx[n.ID] = i
	}
	assert.Less(t, idx[a], idx[b])
	assert.Less(t, idx[a], idx[c])
	assert.Less(t, idx[b], idx[d])
	assert.Less(t, idx[c], idx[d])
}

func TestTopologicalSort_Cycle(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	nodes := []Node{makeNode(a, NodeIdea), makeNode(b, NodeScript)}
	edges := []Edge{makeEdge(a, b), makeEdge(b, a)}
	_, err := TopologicalSort(nodes, edges)
	assert.ErrorIs(t, err, ErrCycleDetected)
}

func TestTopologicalSort_FullPipeline(t *testing.T) {
	// Full video creation pipeline
	ids := make([]uuid.UUID, 11)
	for i := range ids {
		ids[i] = uuid.New()
	}
	nodes := []Node{
		makeNode(ids[0], NodeIdea),
		makeNode(ids[1], NodeScript),
		makeNode(ids[2], NodeCharacter),
		makeNode(ids[3], NodeScene),
		makeNode(ids[4], NodeStoryboard),
		makeNode(ids[5], NodeImagePrompt),
		makeNode(ids[6], NodeImageGeneration),
		makeNode(ids[7], NodeVideoGeneration),
		makeNode(ids[8], NodeAudio),
		makeNode(ids[9], NodeSubtitle),
		makeNode(ids[10], NodeCompose),
	}
	edges := []Edge{
		makeEdge(ids[0], ids[1]),  // idea → script
		makeEdge(ids[1], ids[2]),  // script → character
		makeEdge(ids[1], ids[3]),  // script → scene
		makeEdge(ids[2], ids[4]),  // character → storyboard
		makeEdge(ids[3], ids[4]),  // scene → storyboard
		makeEdge(ids[4], ids[5]),  // storyboard → image_prompt
		makeEdge(ids[5], ids[6]),  // image_prompt → image_generation
		makeEdge(ids[6], ids[7]),  // image_generation → video_generation
		makeEdge(ids[1], ids[8]),  // script → audio
		makeEdge(ids[1], ids[9]),  // script → subtitle
		makeEdge(ids[7], ids[10]), // video_generation → compose
		makeEdge(ids[8], ids[10]), // audio → compose
		makeEdge(ids[9], ids[10]), // subtitle → compose
	}

	// Validate
	assert.NoError(t, ValidateDAG(nodes, edges))

	// Sort
	sorted, err := TopologicalSort(nodes, edges)
	require.NoError(t, err)
	require.Len(t, sorted, 11)

	// Verify all dependencies come before dependents
	idx := make(map[uuid.UUID]int)
	for i, n := range sorted {
		idx[n.ID] = i
	}
	for _, e := range edges {
		assert.Less(t, idx[e.SourceNodeID], idx[e.TargetNodeID],
			"source should come before target in topological order")
	}
}

// ─── Schema Tests ───────────────────────────────────

func TestNodeSchemas_AllTypesHaveSchema(t *testing.T) {
	for _, nt := range AllNodeTypes {
		s := GetNodeSchema(nt)
		assert.NotNil(t, s, "schema should exist for %s", nt)
		assert.Equal(t, nt, s.Type)
		assert.NotEmpty(t, s.Label)
	}
}

func TestNodeSchemas_ComposeHasMultipleInputs(t *testing.T) {
	s := GetNodeSchema(NodeCompose)
	require.NotNil(t, s)
	assert.GreaterOrEqual(t, len(s.Inputs), 2, "compose should have multiple inputs")
}

func TestNodeSchemas_IdeaHasNoInputs(t *testing.T) {
	s := GetNodeSchema(NodeIdea)
	require.NotNil(t, s)
	assert.Empty(t, s.Inputs, "idea should have no inputs")
	assert.NotEmpty(t, s.Outputs)
}

func TestGetNodeSchema_Invalid(t *testing.T) {
	s := GetNodeSchema("nonexistent")
	assert.Nil(t, s)
}
