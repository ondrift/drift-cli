package common

import (
	"fmt"

	"github.com/manifoldco/promptui"
)

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
