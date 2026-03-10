package publishplan

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func ReadFile(path string) (Plan, error) {
	f, err := os.Open(path)
	if err != nil {
		return Plan{}, err
	}
	defer f.Close()

	var p Plan
	p.Env = map[string]string{}

	scanner := bufio.NewScanner(f)
	inEnv := false
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if inEnv {
			if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") {
				k, v, err := parseYAMLKV(strings.TrimSpace(raw))
				if err != nil {
					return Plan{}, fmt.Errorf("%s:%d: %w", path, lineNo, err)
				}
				if strings.TrimSpace(k) == "" {
					return Plan{}, fmt.Errorf("%s:%d: env key cannot be empty", path, lineNo)
				}
				p.Env[k] = unquote(v)
				continue
			}
			inEnv = false
		}

		k, v, err := parseYAMLKV(trimmed)
		if err != nil {
			return Plan{}, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}

		switch k {
		case "version":
			n, err := strconv.Atoi(strings.TrimSpace(unquote(v)))
			if err != nil {
				return Plan{}, fmt.Errorf("%s:%d: invalid version", path, lineNo)
			}
			p.Version = n
		case "service":
			p.Service = unquote(v)
		case "stack":
			p.Stack = unquote(v)
		case "source_dir":
			p.SourceDir = unquote(v)
		case "build_method":
			p.BuildMethod = unquote(v)
		case "image_repo":
			p.ImageRepo = unquote(v)
		case "port":
			n, err := strconv.Atoi(strings.TrimSpace(unquote(v)))
			if err != nil {
				return Plan{}, fmt.Errorf("%s:%d: invalid port", path, lineNo)
			}
			p.Port = n
		case "memory":
			p.Memory = unquote(v)
		case "min_instances":
			n, err := strconv.Atoi(strings.TrimSpace(unquote(v)))
			if err != nil {
				return Plan{}, fmt.Errorf("%s:%d: invalid min_instances", path, lineNo)
			}
			p.MinInstances = &n
		case "allow_unauthenticated":
			b, err := strconv.ParseBool(strings.TrimSpace(strings.ToLower(unquote(v))))
			if err != nil {
				return Plan{}, fmt.Errorf("%s:%d: invalid allow_unauthenticated", path, lineNo)
			}
			p.AllowUnauthenticated = b
		case "env_file":
			p.EnvFile = unquote(v)
		case "env":
			if strings.TrimSpace(v) != "" {
				return Plan{}, fmt.Errorf("%s:%d: env must be a YAML mapping", path, lineNo)
			}
			inEnv = true
		default:
			// Ignore unknown keys for forward compatibility.
		}
	}
	if err := scanner.Err(); err != nil {
		return Plan{}, err
	}

	return Normalize(p)
}

func WriteFile(path string, plan Plan) error {
	p, err := Normalize(plan)
	if err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("version: ")
	b.WriteString(strconv.Itoa(p.Version))
	b.WriteString("\n")
	b.WriteString("service: ")
	b.WriteString(quoteYAML(p.Service))
	b.WriteString("\n")
	b.WriteString("stack: ")
	b.WriteString(quoteYAML(p.Stack))
	b.WriteString("\n")
	b.WriteString("source_dir: ")
	b.WriteString(quoteYAML(p.SourceDir))
	b.WriteString("\n")
	b.WriteString("build_method: ")
	b.WriteString(quoteYAML(p.BuildMethod))
	b.WriteString("\n")
	b.WriteString("image_repo: ")
	b.WriteString(quoteYAML(p.ImageRepo))
	b.WriteString("\n")
	b.WriteString("port: ")
	b.WriteString(strconv.Itoa(p.Port))
	b.WriteString("\n")
	if p.Memory != "" {
		b.WriteString("memory: ")
		b.WriteString(quoteYAML(p.Memory))
		b.WriteString("\n")
	}
	if p.MinInstances != nil {
		b.WriteString("min_instances: ")
		b.WriteString(strconv.Itoa(*p.MinInstances))
		b.WriteString("\n")
	}
	b.WriteString("allow_unauthenticated: ")
	b.WriteString(strconv.FormatBool(p.AllowUnauthenticated))
	b.WriteString("\n")
	if p.EnvFile != "" {
		b.WriteString("env_file: ")
		b.WriteString(quoteYAML(p.EnvFile))
		b.WriteString("\n")
	}

	b.WriteString("env:\n")
	for _, k := range SortedKeys(p.Env) {
		b.WriteString("  ")
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(quoteYAML(p.Env[k]))
		b.WriteString("\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func parseYAMLKV(line string) (string, string, error) {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return "", "", fmt.Errorf("invalid line (expected key: value)")
	}
	k := strings.TrimSpace(line[:i])
	v := strings.TrimSpace(line[i+1:])
	if k == "" {
		return "", "", fmt.Errorf("invalid line (empty key)")
	}
	return k, v, nil
}

func unquote(v string) string {
	v = strings.TrimSpace(v)
	if len(v) < 2 {
		return v
	}
	if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
		u, err := strconv.Unquote(v)
		if err == nil {
			return u
		}
		return v[1 : len(v)-1]
	}
	return v
}

func quoteYAML(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "\"\""
	}
	if strings.ContainsAny(v, ":#\"'{}[]&,*!|>%@`") || strings.Contains(v, " ") {
		return strconv.Quote(v)
	}
	return v
}
