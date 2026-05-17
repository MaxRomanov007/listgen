package config

import (
	"github.com/ilyakaznacheev/cleanenv"
)

const DefaultConfigPath = ".listing.yaml"

const DefaultConfigContent = `# listgen configuration
patterns:
  # Glob patterns to include (empty = include all)
  include: []
    # - "**/*.go"
    # - "**/*.ts"
  # Glob patterns to exclude
  exclude: []
    # - "vendor/**"
    # - "node_modules/**"
  # Respect .gitignore files (including nested ones)
  use_gitignore: true

generating:
  # Output file path
  output: listing.docx

  # Page margins in centimeters
  margin_top: 2
  margin_right: 1
  margin_bottom: 2
  margin_left: 3

  # Fonts
  main_font: "Times New Roman"
  code_font: "Cascadia Mono"
  main_font_size: 14
  code_font_size: 10

  # Indentation in centimeters
  main_indent: 0
  code_indent: 0

  # Line spacing (1.0 = single, 1.5 = one-and-half, 2.0 = double)
  main_line_spacing: 1.5
  code_line_spacing: 1.0

  # Spacing around file title paragraphs
  interval_before_title: true
  interval_after_title: false

  # Draw a border around code blocks
  code_border: true

  # Add a file tree at the beginning of the document
  generate_tree: true

  # Show full relative paths as file titles (false = filename only)
  files_with_path: true
`

type Config struct {
	Patterns   Patterns   `yaml:"patterns"`
	Generating Generating `yaml:"generating"`
}

type Patterns struct {
	Include      []string `yaml:"include"`
	Exclude      []string `yaml:"exclude"`
	UseGitignore bool     `yaml:"use_gitignore" env-default:"true"`
}

type Generating struct {
	Output string `yaml:"output" env-default:"listing.docx"`

	MarginTop    float64 `yaml:"margin_top"    env-default:"2"`
	MarginRight  float64 `yaml:"margin_right"  env-default:"1"`
	MarginBottom float64 `yaml:"margin_bottom" env-default:"2"`
	MarginLeft   float64 `yaml:"margin_left"   env-default:"3"`

	MainFont     string  `yaml:"main_font"      env-default:"Times New Roman"`
	CodeFont     string  `yaml:"code_font"      env-default:"Courier New"`
	MainFontSize float64 `yaml:"main_font_size" env-default:"14"`
	CodeFontSize float64 `yaml:"code_font_size" env-default:"10"`

	MainIndent float64 `yaml:"main_indent" env-default:"0"`
	CodeIndent float64 `yaml:"code_indent" env-default:"0"`

	MainLineSpacing float64 `yaml:"main_line_spacing" env-default:"1.5"`
	CodeLineSpacing float64 `yaml:"code_line_spacing" env-default:"1.0"`

	IntervalBeforeTitle bool `yaml:"interval_before_title" env-default:"true"`
	IntervalAfterTitle  bool `yaml:"interval_after_title"  env-default:"false"`

	CodeBorder    bool `yaml:"code_border"     env-default:"true"`
	GenerateTree  bool `yaml:"generate_tree"   env-default:"true"`
	FilesWithPath bool `yaml:"files_with_path" env-default:"true"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{}
	if err := cleanenv.ReadConfig(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Default() (*Config, error) {
	cfg := &Config{}
	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
