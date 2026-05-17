package scanner

import (
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

type Options struct {
	RootDir         string
	IncludePatterns []string
	ExcludePatterns []string
	UseGitignore    bool
}

func ScanFiles(opts Options) ([]string, error) {
	// Кэш: абсолютный путь директории → скомпилированный .gitignore (nil = файла нет)
	giCache := map[string]*gitignore.GitIgnore{}

	var files []string

	err := filepath.WalkDir(opts.RootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// .git всегда пропускаем
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		// Для каждой директории пробуем загрузить её .gitignore
		if opts.UseGitignore {
			dir := path
			if !d.IsDir() {
				dir = filepath.Dir(path)
			}
			if _, loaded := giCache[dir]; !loaded {
				gi, err := gitignore.CompileIgnoreFile(filepath.Join(dir, ".gitignore"))
				if err == nil {
					giCache[dir] = gi
				} else {
					giCache[dir] = nil // файла нет — запоминаем, чтобы не проверять снова
				}
			}

			if isIgnored(giCache, opts.RootDir, path, d.IsDir()) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(opts.RootDir, path)
		if err != nil {
			return err
		}

		// Применяем паттерны exclude
		for _, pattern := range opts.ExcludePatterns {
			matched, err := matchPattern(pattern, rel)
			if err != nil {
				return err
			}
			if matched {
				return nil
			}
		}

		// Применяем паттерны include
		if len(opts.IncludePatterns) > 0 {
			matched := false
			for _, pattern := range opts.IncludePatterns {
				ok, err := matchPattern(pattern, rel)
				if err != nil {
					return err
				}
				if ok {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

// isIgnored проверяет path против всех .gitignore от корня до директории файла.
// Каждый .gitignore применяется относительно своей директории — так же, как это делает git.
func isIgnored(cache map[string]*gitignore.GitIgnore, root, path string, isDir bool) bool {
	dir := path
	if !isDir {
		dir = filepath.Dir(path)
	}

	// Поднимаемся от директории файла вверх до root,
	// проверяя каждый найденный .gitignore
	for {
		if gi, ok := cache[dir]; ok && gi != nil {
			rel, err := filepath.Rel(dir, path)
			if err == nil && gi.MatchesPath(rel) {
				return true
			}
		}

		if dir == root {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir { // дошли до корня ФС
			break
		}
		dir = parent
	}

	return false
}

// matchPattern поддерживает glob-паттерны вида **/*.go, *.txt, src/**
func matchPattern(pattern, path string) (bool, error) {
	path = filepath.ToSlash(path)
	pattern = filepath.ToSlash(pattern)

	matched, err := filepath.Match(pattern, path)
	if err != nil {
		return false, err
	}
	if matched {
		return true, nil
	}

	if strings.Contains(pattern, "**") {
		return matchDoublestar(pattern, path), nil
	}

	// Пробуем матчить только по имени файла
	matched, err = filepath.Match(pattern, filepath.Base(path))
	return matched, err
}

func matchDoublestar(pattern, path string) bool {
	parts := strings.SplitN(pattern, "**", 2)
	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := strings.TrimPrefix(parts[1], "/")

	if prefix != "" && !strings.HasPrefix(path, prefix) {
		return false
	}

	if suffix == "" {
		return true
	}

	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.TrimPrefix(trimmed, "/")

	matched, _ := filepath.Match(suffix, filepath.Base(trimmed))
	if matched {
		return true
	}

	matched, _ = filepath.Match(suffix, trimmed)
	return matched
}
