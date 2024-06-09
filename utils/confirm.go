package utils

import (
	"bufio"
	"os"
	"strings"

	"gofish/logs"
)

func Confirm(prompt string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	logs.Logs().Info().Msg(prompt)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "y" {
			return true, nil
		} else if input == "n" {
			return false, nil
		} else {
			logs.Logs().Warn().Msgf("Invalid input '%s'. Use 'y' or 'n'", input)
		}
	}
}
