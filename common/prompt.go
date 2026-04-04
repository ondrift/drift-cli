package common

import (
	"fmt"

	"github.com/manifoldco/promptui"
)

func PromptForMultiSelect(label string, items []string) []string {
	selected := make(map[int]bool)

	for {
		displayItems := make([]string, len(items))
		for i, item := range items {
			prefix := " "
			if selected[i] {
				prefix = "✔"
			}
			displayItems[i] = fmt.Sprintf("%s %s", prefix, item)
		}

		prompt := promptui.Select{
			Label: label,
			Items: displayItems,
			Size:  len(displayItems),
		}

		i, _, err := prompt.Run()
		if err != nil {
			// Enter confirms selection if at least one is chosen
			if len(selected) > 0 {
				break
			} else {
				fmt.Println("❌ No selection made. Try again.")
				continue
			}
		}

		selected[i] = !selected[i] // toggle selection
	}

	results := []string{}
	for i, sel := range selected {
		if sel {
			results = append(results, items[i])
		}
	}
	return results
}

func PromptForSelect(label string, items []string) string {
	prompt := promptui.Select{
		Label: label,
		Items: items,
		Size:  len(items),
	}
	_, result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return ""
	}
	return result
}

func PromptForInput(label string) string {
	prompt := promptui.Prompt{
		Label: label,
	}
	result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return ""
	}
	return result
}

func PromptForInputHidden(label string) string {
	prompt := promptui.Prompt{
		Mask:  '*',
		Label: label,
	}
	result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return ""
	}
	return result
}
