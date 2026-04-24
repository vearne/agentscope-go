package message

type Source map[string]interface{}

func NewBase64Source(mediaType, data string) Source {
	return Source{
		"type":       "base64",
		"media_type": mediaType,
		"data":       data,
	}
}

func NewURLSource(url string) Source {
	return Source{
		"type": "url",
		"url":  url,
	}
}

func GetSourceType(s Source) string {
	if v, ok := s["type"]; ok {
		return v.(string)
	}
	return ""
}

func GetSourceURL(s Source) string {
	if v, ok := s["url"]; ok {
		return v.(string)
	}
	return ""
}

func GetSourceData(s Source) string {
	if v, ok := s["data"]; ok {
		return v.(string)
	}
	return ""
}

func GetSourceMediaType(s Source) string {
	if v, ok := s["media_type"]; ok {
		return v.(string)
	}
	return ""
}
