# Презентация к защите ВКР

Итоговые файлы:

- `Infrastructure-Management-Platform-defense.pptx` — основная презентация.
- `Infrastructure-Management-Platform-defense.pdf` — экспорт в PDF.
- `preview/` — PNG-превью слайдов для быстрой проверки.
- `generate_presentation.ps1` — воспроизводимый генератор PPTX/PDF через Microsoft PowerPoint.
- `assets/mai-logo-1.png` — отрендеренный логотип МАИ для титульного слайда.

## Основа содержания

Содержание собрано по материалам проекта и ВКР:

- `diploma-text/diploma_latex_template/main.tex`
- `diploma-text/diploma_latex_template/title-page.txt`
- главы ВКР из `diploma-text/diploma_latex_template/contents/`
- документация `docs/`
- архитектурные диаграммы `diploma-text/platform-arch-images/`
- benchmark-отчет `docs/03-develop/02-benchmarks/report-2026-04-24.md`
- QR-код `diploma-text/repository_qr_code.png`

## Визуальная основа

Композиция и академичный тон адаптированы по примеру:

- `presentation/презы_Т.pdf`

Дополнительный внешний референс:

- Slidesgo: `Commercial Engineering Thesis Defense`
  `https://slidesgo.com/theme/commercial-engineering-thesis-defense`

Итоговый стиль: светлая академичная основа в духе примера `презы_Т.pdf`, графитовые инженерные панели и синий акцент МАИ / infrastructure-management тематики.

## Повторная сборка

На Windows PowerShell безопаснее запускать генератор через явное чтение UTF-8:

```powershell
$path = Resolve-Path '.\presentation\infrastructure-vkr-defense\generate_presentation.ps1'
$root = Split-Path -Parent $path
$text = [System.IO.File]::ReadAllText($path, [System.Text.Encoding]::UTF8)
& ([scriptblock]::Create($text)) -ScriptRootOverride $root -OutputDir $root
```

Для экспорта нужен установленный Microsoft PowerPoint.
