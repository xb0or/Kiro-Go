package proxy

func expandPreviousResponseHistory(prev *ResponsesObject) []OpenAIMessage {
	if prev == nil {
		return nil
	}

	messages := make([]OpenAIMessage, 0)

	if prior, err := parseResponsesInput(prev.StoredInput); err == nil {
		messages = append(messages, prior...)
	}

	for _, item := range prev.Output {
		switch item.Type {
		case "message":
			text := joinTextParts(item.Content)
			role := item.Role
			if role == "" {
				role = "assistant"
			}
			if text == "" && role == "assistant" {
				continue
			}
			messages = append(messages, OpenAIMessage{
				Role:    role,
				Content: text,
			})
		case "function_call":
			tc := ToolCall{
				ID:   item.CallID,
				Type: "function",
			}
			if tc.ID == "" {
				tc.ID = item.ID
			}
			tc.Function.Name = item.Name
			tc.Function.Arguments = item.Arguments
			messages = append(messages, OpenAIMessage{
				Role:      "assistant",
				Content:   "",
				ToolCalls: []ToolCall{tc},
			})
		}
	}

	return messages
}

func joinTextParts(parts []ResponseContentPart) string {
	if len(parts) == 0 {
		return ""
	}
	out := ""
	for _, p := range parts {
		if p.Type == "output_text" || p.Type == "text" || p.Type == "input_text" {
			out += p.Text
		}
	}
	return out
}
