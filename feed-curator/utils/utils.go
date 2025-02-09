package utils 

import (
    "fmt"
    "time"
    "log"
    "strings"

    "github.com/google/generative-ai-go/genai"
)

func LogMessage(message, color string, errs ...error) {

    var err error
	if len(errs) > 0 {
		err = errs[0]
	} else {
		err = nil
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
    
    log.Printf("[CURATOR_LOGS] [%s] %s ERROR: %v", timestamp, message, err)

    if color == "red" {
        fmt.Printf("\033[31m [%s] %s \033[0m ERROR: %v\n", timestamp, message, err)
    } else if color == "green" {
        fmt.Printf("\033[32m [%s] %s \033[0m ERROR: %v\n", timestamp, message, err)
    } else {
        fmt.Printf("\033[31m [%s] %s \033[0m ERROR: %v\n", timestamp, message, err)
    }

}

func PrintResponse(resp *genai.GenerateContentResponse) {
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				fmt.Println(part)
			}
		}
	}
}

func RemoveDuplicates(input []string) ([]string, error) {
    if input == nil {
        return nil, nil
    }
    
    // Use map to track seen elements
    seen := make(map[string]struct{})
    
    // Create result slice with initial capacity matching input
    result := make([]string, 0, len(input))
    
    // Iterate through input preserving order
    for _, str := range input {
        // If string hasn't been seen before, add it to result
        if _, exists := seen[str]; !exists {
            seen[str] = struct{}{}
            result = append(result, str)
        }
    }
    
    return result, nil
}

func RemoveHashPrefix(input []string) ([]string, error) {
    if input == nil {
        return nil, nil
    }
    
    // Create result slice (capacity will be adjusted as needed)
    result := make([]string, 0, len(input))
    
    // Keep only strings that don't start with "##"
    for _, str := range input {
        // Trim leading whitespace for checking prefix
        trimmed := strings.TrimSpace(str)
        if !strings.HasPrefix(trimmed, "##") {
            result = append(result, str)
        }
    }
    
    return result, nil
}

func ExtractPoints(input string) []string {
	var points []string
	lines := strings.Split(input, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "*") {
			points = append(points, strings.TrimSpace(trimmed[1:]))
		}
	}

	return points
}

func FindHighlightIndexes(content string, highlights []string) [][]int {
	var result [][]int

	for _, highlight := range highlights {
		start := 0
		for start < len(content) {
			start = strings.Index(content[start:], highlight)
			if start == -1 {
				break
			}
			start += len(result) // Adjust start position by previous offset
			end := start + len(highlight)
			result = append(result, []int{start, end})
			start = end // Move the cursor forward
		}
	}
	return result
}
