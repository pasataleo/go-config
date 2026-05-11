package flags

import "strings"

// Parse parses the arguments and extracts any flags from them, returning the remaining arguments and the flag
// values keyed by flag name.
func Parse(args []string) ([]string, map[string][]string) {
	values := make(map[string][]string)
	var remainingArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// "--" signals the end of flags; everything after is positional.
		if arg == "--" {
			remainingArgs = append(remainingArgs, args[i:]...)
			break
		}

		if isFlagName(arg) {
			// It's a flag
			flagName := ""
			flagValue := ""

			// Remove prefix
			if strings.HasPrefix(arg, "--") {
				flagName = arg[2:]
			} else {
				flagName = arg[1:]
			}

			// Check for equals sign
			if strings.Contains(flagName, "=") {
				parts := strings.SplitN(flagName, "=", 2)
				flagName = parts[0]
				flagValue = parts[1]
			} else {
				// Check next argument for value
				if i+1 < len(args) && !isFlagName(args[i+1]) {
					flagValue = args[i+1]
					i++ // Skip the next argument as it's the value
				} else {
					flagValue = ""
				}
			}

			values[flagName] = append(values[flagName], flagValue)
		} else {
			// Not a flag, preserve it
			remainingArgs = append(remainingArgs, arg)
		}
	}

	return remainingArgs, values
}

// isFlagName judges whether a string is a valid flag name.
func isFlagName(name string) bool {
	if strings.HasPrefix(name, "--") {
		return len(name) > 2
	}

	if strings.HasPrefix(name, "-") {
		return len(name) > 1
	}

	return false
}
