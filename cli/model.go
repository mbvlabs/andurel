package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mbvlabs/andurel/generator"
)

func runModelUpdate(resourceName string, autoApply bool) error {
	gen, err := generator.New()
	if err != nil {
		return err
	}

	result, err := gen.UpdateModel(resourceName)
	if err != nil {
		return err
	}

	if !result.HasChanges {
		fmt.Println("No changes — model struct is already up to date.")
		return nil
	}

	diff, err := result.Diff()
	if err != nil {
		return fmt.Errorf("failed to compute diff: %w", err)
	}

	fmt.Printf("Changes to %s:\n\n", result.ModelPath)
	printColoredDiff(diff)
	fmt.Println()

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

	fmt.Printf("Updated %s\n", result.ModelPath)
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
