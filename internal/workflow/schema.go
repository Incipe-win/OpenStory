package workflow

// NodeSchema defines the input/output contract for a node type.
type NodeSchema struct {
	Type        NodeType  `json:"type"`
	Label       string    `json:"label"`
	Description string    `json:"description"`
	Inputs      []PortDef `json:"inputs"`
	Outputs     []PortDef `json:"outputs"`
}

// PortDef defines a named input/output port on a node.
type PortDef struct {
	Handle   string `json:"handle"`
	Label    string `json:"label"`
	DataType string `json:"data_type"` // text, json, image_url, video_url, audio_url
	Required bool   `json:"required"`
}

// NodeSchemas maps each node type to its schema.
var NodeSchemas = map[NodeType]NodeSchema{
	NodeIdea: {
		Type: NodeIdea, Label: "创意输入", Description: "用户输入创意文本",
		Inputs:  nil,
		Outputs: []PortDef{{Handle: "text", Label: "创意文本", DataType: "text"}},
	},
	NodeScript: {
		Type: NodeScript, Label: "剧本生成", Description: "根据创意生成剧本",
		Inputs:  []PortDef{{Handle: "idea", Label: "创意", DataType: "text", Required: true}},
		Outputs: []PortDef{{Handle: "script", Label: "剧本 JSON", DataType: "json"}},
	},
	NodeCharacter: {
		Type: NodeCharacter, Label: "角色设计", Description: "根据剧本生成角色描述",
		Inputs:  []PortDef{{Handle: "script", Label: "剧本", DataType: "json", Required: true}},
		Outputs: []PortDef{{Handle: "characters", Label: "角色列表", DataType: "json"}},
	},
	NodeScene: {
		Type: NodeScene, Label: "场景设计", Description: "根据剧本拆分场景",
		Inputs:  []PortDef{{Handle: "script", Label: "剧本", DataType: "json", Required: true}},
		Outputs: []PortDef{{Handle: "scenes", Label: "场景列表", DataType: "json"}},
	},
	NodeStoryboard: {
		Type: NodeStoryboard, Label: "分镜生成", Description: "根据场景和角色生成分镜",
		Inputs: []PortDef{
			{Handle: "scenes", Label: "场景", DataType: "json", Required: true},
			{Handle: "characters", Label: "角色", DataType: "json", Required: true},
		},
		Outputs: []PortDef{{Handle: "storyboard", Label: "分镜列表", DataType: "json"}},
	},
	NodeImagePrompt: {
		Type: NodeImagePrompt, Label: "图片提示词", Description: "根据分镜生成图片提示词",
		Inputs:  []PortDef{{Handle: "storyboard", Label: "分镜", DataType: "json", Required: true}},
		Outputs: []PortDef{{Handle: "prompts", Label: "提示词列表", DataType: "json"}},
	},
	NodeImageGeneration: {
		Type: NodeImageGeneration, Label: "图片生成", Description: "调用模型生成图片",
		Inputs:  []PortDef{{Handle: "prompts", Label: "提示词", DataType: "json", Required: true}},
		Outputs: []PortDef{{Handle: "images", Label: "图片 URL 列表", DataType: "json"}},
	},
	NodeVideoGeneration: {
		Type: NodeVideoGeneration, Label: "视频生成", Description: "图片转视频",
		Inputs:  []PortDef{{Handle: "images", Label: "图片", DataType: "json", Required: true}},
		Outputs: []PortDef{{Handle: "videos", Label: "视频 URL 列表", DataType: "json"}},
	},
	NodeAudio: {
		Type: NodeAudio, Label: "音频生成", Description: "TTS 或背景音乐生成",
		Inputs:  []PortDef{{Handle: "script", Label: "剧本", DataType: "json", Required: true}},
		Outputs: []PortDef{{Handle: "audio", Label: "音频 URL", DataType: "audio_url"}},
	},
	NodeSubtitle: {
		Type: NodeSubtitle, Label: "字幕生成", Description: "根据剧本生成 SRT 字幕",
		Inputs:  []PortDef{{Handle: "script", Label: "剧本", DataType: "json", Required: true}},
		Outputs: []PortDef{{Handle: "subtitle", Label: "字幕 SRT", DataType: "text"}},
	},
	NodeCompose: {
		Type: NodeCompose, Label: "视频合成", Description: "FFmpeg 合成最终视频",
		Inputs: []PortDef{
			{Handle: "videos", Label: "视频片段", DataType: "json", Required: true},
			{Handle: "audio", Label: "音频", DataType: "audio_url", Required: false},
			{Handle: "subtitle", Label: "字幕", DataType: "text", Required: false},
		},
		Outputs: []PortDef{{Handle: "video", Label: "最终视频 URL", DataType: "video_url"}},
	},
}

// GetNodeSchema returns the schema for a given node type, or nil if not found.
func GetNodeSchema(t NodeType) *NodeSchema {
	s, ok := NodeSchemas[t]
	if !ok {
		return nil
	}
	return &s
}
