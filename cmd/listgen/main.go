package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/MaxRomanov007/listgen/internal/config"
	"github.com/MaxRomanov007/listgen/internal/generator"
	"github.com/MaxRomanov007/listgen/internal/scanner"
)

const usage = `Usage:
  listgen <command> [flags] [directory]

Commands:
  generate, gen   Generate a .docx listing (default if no command given)
  init            Create a default .listing.yaml in the current directory

Flags for generate:
  -c, --config <path>   Config file path (default: .listing.yaml)
  -f, --force           Overwrite output file if it already exists

Examples:
  listgen init
  listgen gen
  listgen gen -f
  listgen gen --config myconfig.yaml /path/to/project
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return cmdGenerate(os.Args[1:])
	}

	switch os.Args[1] {
	case "generate", "gen":
		return cmdGenerate(os.Args[2:])
	case "init":
		return cmdInit()
	case "-h", "--help", "help":
		fmt.Print(usage)
		return nil
	default:
		// Если первый аргумент не субкоманда — считаем, что это generate
		return cmdGenerate(os.Args[1:])
	}
}

// cmdGenerate сканирует файлы и генерирует .docx.
func cmdGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)

	var configPath string
	fs.StringVar(&configPath, "config", config.DefaultConfigPath, "path to config file")
	fs.StringVar(&configPath, "c", config.DefaultConfigPath, "path to config file (shorthand)")

	var force bool
	fs.BoolVar(&force, "force", false, "overwrite output file if it exists")
	fs.BoolVar(&force, "f", false, "overwrite output file if it exists (shorthand)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Корневая директория — первый позиционный аргумент или текущая
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	if rest := fs.Args(); len(rest) > 0 {
		rootDir = rest[0]
	}

	// Загружаем конфиг
	cfg, err := config.Load(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("Config file %q not found, using defaults\n", configPath)
			cfg, err = config.Default()
			if err != nil {
				return fmt.Errorf("creating default config: %w", err)
			}
		} else {
			return fmt.Errorf("loading config from %q: %w", configPath, err)
		}
	}

	// Проверяем, не существует ли уже выходной файл
	outputPath := cfg.Generating.Output
	if _, err := os.Stat(outputPath); err == nil {
		if !force {
			return fmt.Errorf(
				"output file %q already exists, use --force / -f to overwrite",
				outputPath,
			)
		}
		if err := os.Remove(outputPath); err != nil {
			return fmt.Errorf("removing existing output file: %w", err)
		}
	}

	fmt.Printf("Scanning: %s\n", rootDir)

	files, err := scanner.ScanFiles(scanner.Options{
		RootDir:         rootDir,
		IncludePatterns: cfg.Patterns.Include,
		ExcludePatterns: cfg.Patterns.Exclude,
		UseGitignore:    cfg.Patterns.UseGitignore,
	})
	if err != nil {
		return fmt.Errorf("scanning files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found in %s", rootDir)
	}

	fmt.Printf("Found %d file(s)\n", len(files))

	if err := generator.Generate(rootDir, files, cfg); err != nil {
		return fmt.Errorf("generating document: %w", err)
	}

	return nil
}

// cmdInit создаёт .listing.yaml с настройками по умолчанию в текущей директории.
func cmdInit() error {
	if _, err := os.Stat(config.DefaultConfigPath); err == nil {
		return fmt.Errorf("%q already exists", config.DefaultConfigPath)
	}

	if err := os.WriteFile(
		config.DefaultConfigPath,
		[]byte(config.DefaultConfigContent),
		0o644,
	); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Printf("Created %s\n", config.DefaultConfigPath)
	return nil
}
