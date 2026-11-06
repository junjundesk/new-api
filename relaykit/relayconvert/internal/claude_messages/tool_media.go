package claudemessages

import (
	"strings"

	"github.com/QuantumNous/new-api/relaykit/dto"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
)

const (
	// toolMediaPlaceholder 是媒体被迁出 tool 消息后留在原位的占位文本。
	toolMediaPlaceholder = "[media moved to the following user message]"
	// toolMediaDataURLThreshold 超过该长度的 data URL 字符串会被当作媒体抽取，
	// 避免把整段 base64 留在 tool 文本里。
	toolMediaDataURLThreshold = 1024
)

// buildImageMedia 把图片字段组装成 OpenAI 的 image_url 内容块。
// url 优先；否则用 media_type（缺省 image/png）与 base64 data 拼 data URL。
// 无法组装时返回 nil，调用方应跳过该图片。
func buildImageMedia(url, mediaType, data, detail string) *dto.MediaContent {
	if url == "" {
		if mediaType == "" {
			mediaType = "image/png"
		}
		if !strings.HasPrefix(mediaType, "image/") {
			return nil
		}
		if data == "" {
			return nil
		}
		if strings.HasPrefix(data, "data:") {
			url = data
		} else {
			url = "data:" + mediaType + ";base64," + data
		}
	}
	return &dto.MediaContent{
		Type: "image_url",
		ImageUrl: &dto.MessageImageUrl{
			Url:      url,
			Detail:   detail,
			MimeType: mediaType,
		},
	}
}

// dataURLToMedia 把超长的 data:image/... 字符串转成 image_url 媒体块。
func dataURLToMedia(value string) *dto.MediaContent {
	if len(value) <= toolMediaDataURLThreshold || !strings.HasPrefix(value, "data:image/") {
		return nil
	}
	comma := strings.Index(value, ",")
	if comma < 0 {
		return nil
	}
	mediaType := value[len("data:"):comma]
	if i := strings.Index(mediaType, ";"); i >= 0 {
		mediaType = mediaType[:i]
	}
	if !strings.HasPrefix(mediaType, "image/") {
		return nil
	}
	return buildImageMedia(value, mediaType, "", "")
}

// planToolResultMedia 把 tool_result 的 content 拆成 tool 消息文本与被抽取的媒体块。
// tool 消息只能承载文本，图片等媒体需要由调用方迁移到后续的 user 消息中。
// 没有任何媒体时退回原有的 JSON 序列化结果，保证输出字节稳定。
func planToolResultMedia(content any) (string, []dto.MediaContent) {
	switch v := content.(type) {
	case nil:
		return "", nil
	case string:
		if media := dataURLToMedia(v); media != nil {
			return toolMediaPlaceholder, []dto.MediaContent{*media}
		}
		return v, nil
	}

	mediaParts := make([]dto.MediaContent, 0)
	text, found := collectToolMedia(content, &mediaParts)
	if found {
		return text, mediaParts
	}

	// 无媒体时保持既有序列化行为（[]dto.ClaudeMediaMessage -> JSON 字符串）。
	parsed, _ := kitutil.Any2Type[[]dto.ClaudeMediaMessage](content)
	encoded, _ := kitutil.Marshal(parsed)
	if len(encoded) > 0 && string(encoded) != "null" {
		return string(encoded), nil
	}
	// 非数组形状（对象/数字等）退回原值，避免输出 "null"。
	return marshalUnknown(content), nil
}

// collectToolMedia 递归遍历 tool_result content，抽取图片并把原位替换为占位文本。
// 返回拼好的文本与是否找到媒体。
func collectToolMedia(content any, media *[]dto.MediaContent) (string, bool) {
	switch v := content.(type) {
	case string:
		return v, false
	case []any:
		parts := make([]string, 0, len(v))
		found := false
		for _, item := range v {
			text, itemFound := collectToolMedia(item, media)
			found = found || itemFound
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n"), found
	case map[string]any:
		return collectToolMediaMap(v, media)
	default:
		return marshalUnknown(content), false
	}
}

func collectToolMediaMap(item map[string]any, media *[]dto.MediaContent) (string, bool) {
	typeName, _ := item["type"].(string)
	switch typeName {
	case "image":
		if mediaContent := imageBlockToMedia(item); mediaContent != nil {
			*media = append(*media, *mediaContent)
			return toolMediaPlaceholder, true
		}
		return marshalUnknown(item), false
	case "input_image":
		if mediaContent := inputImageBlockToMedia(item); mediaContent != nil {
			*media = append(*media, *mediaContent)
			return toolMediaPlaceholder, true
		}
		return marshalUnknown(item), false
	}
	// 嵌套的 content（tool_result 或任意携带 content 的对象）。
	if inner, ok := item["content"]; ok {
		if text, found := collectToolMedia(inner, media); found || text != "" {
			return text, found
		}
	}
	if text, ok := item["text"].(string); ok {
		return text, false
	}
	return marshalUnknown(item), false
}

// imageBlockToMedia 处理 Anthropic 风格的 image 块（source 内包含 url/data）。
func imageBlockToMedia(item map[string]any) *dto.MediaContent {
	url := kitutil.Interface2String(item["url"])
	mediaType := kitutil.Interface2String(item["media_type"])
	data := ""
	if source, ok := item["source"].(map[string]any); ok {
		url = firstNonEmpty(kitutil.Interface2String(source["url"]), url)
		mediaType = firstNonEmpty(kitutil.Interface2String(source["media_type"]), mediaType)
		data = kitutil.Interface2String(source["data"])
	}
	return buildImageMedia(url, mediaType, data, kitutil.Interface2String(item["detail"]))
}

// inputImageBlockToMedia 处理 OpenAI 风格的 input_image 块。
func inputImageBlockToMedia(item map[string]any) *dto.MediaContent {
	url := kitutil.Interface2String(item["url"])
	detail := kitutil.Interface2String(item["detail"])
	switch imageURL := item["image_url"].(type) {
	case string:
		url = firstNonEmpty(imageURL, url)
	case map[string]any:
		url = firstNonEmpty(kitutil.Interface2String(imageURL["url"]), url)
		detail = firstNonEmpty(kitutil.Interface2String(imageURL["detail"]), detail)
	}
	return buildImageMedia(url, kitutil.Interface2String(item["media_type"]), "", detail)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func marshalUnknown(value any) string {
	encoded, err := kitutil.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}
