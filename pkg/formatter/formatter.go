package formatter

import (
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
)

type FormatterBase interface {
	Format(msgs []*message.Msg) ([]model.FormattedMessage, error)
}

func assertListOfMsgs(msgs []*message.Msg) {
	if msgs == nil {
		panic("messages must not be nil")
	}
}

func convertToolResultOutput(output interface{}) (string, []imageRef) {
	switch v := output.(type) {
	case string:
		return v, nil
	case []message.ContentBlock:
		return convertToolResultBlocks(v)
	default:
		return "", nil
	}
}

type imageRef struct {
	URL   string
	Block message.ContentBlock
}

func convertToolResultBlocks(blocks []message.ContentBlock) (string, []imageRef) {
	var textual []string
	var images []imageRef
	for _, b := range blocks {
		switch message.GetBlockType(b) {
		case message.BlockText:
			textual = append(textual, message.GetBlockText(b))
		case message.BlockImage:
			src := b["source"]
			if s, ok := src.(message.Source); ok {
				url := ""
				if message.GetSourceType(s) == "url" {
					url = message.GetSourceURL(s)
					textual = append(textual, "The returned image can be found at: "+url)
				} else if message.GetSourceType(s) == "base64" {
					textual = append(textual, "The returned image is provided as base64 data")
				}
				images = append(images, imageRef{URL: url, Block: b})
			}
		}
	}

	if len(textual) == 1 {
		return textual[0], images
	}

	result := ""
	for i, t := range textual {
		if i > 0 {
			result += "\n"
		}
		result += "- " + t
	}
	return result, images
}
