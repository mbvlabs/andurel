package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func runModelUpdate(resourceName string, autoApply bool, skipFactory bool) error {
	gen, err := newGenerator()
	if err != nil {
		return err
	}

	result, err := gen.UpdateModel(resourceName)
	if err != nil {
		return err
	}
	if skipFactory {
		result.FactoryPath = ""
		result.OldFactoryContent = ""
		result.NewFactoryContent = ""
		result.FactoryHasChanges = false
	}

	if !result.HasChanges && !result.FactoryHasChanges {
		fmt.Println("No changes — model is already up to date.")
		return nil
	}

	// Show model diff if there are changes
	if result.HasChanges {
		diff, err := result.Diff()
		if err != nil {
			return fmt.Errorf("failed to compute diff: %w", err)
		}

		fmt.Printf("Changes to %s:\n\n", result.ModelPath)
		printColoredDiff(diff)
		fmt.Println()
	}

	// Show factory diff if there are changes
	if result.FactoryHasChanges {
		factoryDiff, err := result.FactoryDiff()
		if err != nil {
			return fmt.Errorf("failed to compute factory diff: %w", err)
		}

		fmt.Printf("Changes to %s:\n\n", result.FactoryPath)
		printColoredDiff(factoryDiff)
		fmt.Println()
	}

	if !autoApply {
		confirmed, err := confirmModelApply()
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := gen.ApplyModelUpdate(result); err != nil {
		return err
	}

	if result.HasChanges {
		fmt.Printf("Updated %s\n", result.ModelPath)
	}
	if result.FactoryHasChanges {
		fmt.Printf("Updated %s\n", result.FactoryPath)
	}
	return nil
}

func confirmModelApply() (bool, error) {
	fmt.Print("Apply these changes? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func printColoredDiff(diff string) {
	for line := range strings.SplitSeq(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			fmt.Println(line)
		case strings.HasPrefix(line, "+"):
			fmt.Printf("\033[32m%s\033[0m\n", line)
		case strings.HasPrefix(line, "-"):
			fmt.Printf("\033[31m%s\033[0m\n", line)
		case strings.HasPrefix(line, "@@"):
			fmt.Printf("\033[36m%s\033[0m\n", line)
		default:
			fmt.Println(line)
		}
	}
}
