package message

type BlockType string

const (
	BlockText       BlockType = "text"
	BlockThinking   BlockType = "thinking"
	BlockToolUse    BlockType = "tool_use"
	BlockToolResult BlockType = "tool_result"
	BlockImage      BlockType = "image"
	BlockAudio      BlockType = "audio"
	BlockVideo      BlockType = "video"
)

type ContentBlock map[string]interface{}

func NewTextBlock(text string) ContentBlock {
	return ContentBlock{
		"type": string(BlockText),
		"text": text,
	}
}

func NewThinkingBlock(thinking string) ContentBlock {
	return ContentBlock{
		"type":      string(BlockThinking),
		"thinking":  thinking,
	}
}

func NewToolUseBlock(id, name string, input interface{}) ContentBlock {
	return ContentBlock{
		"type":  string(BlockToolUse),
		"id":    id,
		"name":  name,
		"input": input,
	}
}

func NewToolResultBlock(id string, output interface{}, isError bool) ContentBlock {
	return ContentBlock{
		"type":    string(BlockToolResult),
		"id":      id,
		"output":  output,
		"is_error": isError,
	}
}

func NewImageBlock(source Source) ContentBlock {
	return ContentBlock{
		"type":   string(BlockImage),
		"source": source,
	}
}

func NewAudioBlock(source Source) ContentBlock {
	return ContentBlock{
		"type":   string(BlockAudio),
		"source": source,
	}
}

func NewVideoBlock(source Source) ContentBlock {
	return ContentBlock{
		"type":   string(BlockVideo),
		"source": source,
	}
}

func GetBlockType(block ContentBlock) BlockType {
	if v, ok := block["type"]; ok {
		return BlockType(v.(string))
	}
	return ""
}

func GetBlockText(block ContentBlock) string {
	if v, ok := block["text"]; ok {
		return v.(string)
	}
	return ""
}

func GetBlockThinking(block ContentBlock) string {
	if v, ok := block["thinking"]; ok {
		return v.(string)
	}
	return ""
}

func GetBlockToolUseID(block ContentBlock) string {
	if v, ok := block["id"]; ok {
		return v.(string)
	}
	return ""
}

func GetBlockToolUseName(block ContentBlock) string {
	if v, ok := block["name"]; ok {
		return v.(string)
	}
	return ""
}

func GetBlockToolUseInput(block ContentBlock) interface{} {
	if v, ok := block["input"]; ok {
		return v
	}
	return nil
}

func GetBlockToolResultID(block ContentBlock) string {
	if v, ok := block["id"]; ok {
		return v.(string)
	}
	return ""
}

func GetBlockToolResultOutput(block ContentBlock) interface{} {
	if v, ok := block["output"]; ok {
		return v
	}
	return nil
}

func GetBlockToolResultIsError(block ContentBlock) bool {
	if v, ok := block["is_error"]; ok {
		if b, ok2 := v.(bool); ok2 {
			return b
		}
	}
	return false
}

func IsTextBlock(block ContentBlock) bool       { return GetBlockType(block) == BlockText }
func IsThinkingBlock(block ContentBlock) bool   { return GetBlockType(block) == BlockThinking }
func IsToolUseBlock(block ContentBlock) bool    { return GetBlockType(block) == BlockToolUse }
func IsToolResultBlock(block ContentBlock) bool { return GetBlockType(block) == BlockToolResult }
func IsImageBlock(block ContentBlock) bool      { return GetBlockType(block) == BlockImage }
func IsAudioBlock(block ContentBlock) bool      { return GetBlockType(block) == BlockAudio }
func IsVideoBlock(block ContentBlock) bool      { return GetBlockType(block) == BlockVideo }
