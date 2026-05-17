# listgen

CLI утилита для генерации Word (.docx) документа из исходных файлов проекта.

## Установка

```bash
go install github.com/user/listgen/cmd/listgen@latest
```

Или локально:

```bash
git clone ...
cd listgen
go build -o listgen ./cmd/listgen
```

## Использование

```bash
# Текущая директория, конфиг .listing.yaml
listgen

# Указать директорию
listgen /path/to/project

# Указать кастомный конфиг
listgen --config myconfig.yaml
listgen -c myconfig.yaml /path/to/project
```

## Конфиг (.listing.yaml)

```yaml
patterns:
  include:
    - "**/*.go"
    - "**/*.ts"
  exclude:
    - "vendor/**"
  use_gitignore: true   # учитывать .gitignore

generating:
  output: listing.docx

  margin_top: 2         # поля в сантиметрах
  margin_right: 1
  margin_bottom: 2
  margin_left: 3

  main_font: "Times New Roman"
  code_font: "Courier New"
  main_font_size: 14
  code_font_size: 10

  main_line_spacing: 1.5
  code_line_spacing: 1.0

  code_border: true     # граница вокруг блоков кода
  generate_tree: true   # дерево файлов в начале документа
  files_with_path: true # полный относительный путь в заголовке
```

Если конфиг не найден — используются значения по умолчанию.