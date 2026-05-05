param(
    [string]$OutputDir = "",
    [string]$ScriptRootOverride = ""
)

$ErrorActionPreference = "Stop"

function RgbColor([int]$r, [int]$g, [int]$b) {
    return $r + ($g * 256) + ($b * 65536)
}

function Add-TextBox {
    param(
        [object]$Slide,
        [string]$Text,
        [double]$Left,
        [double]$Top,
        [double]$Width,
        [double]$Height,
        [int]$FontSize = 18,
        [int]$Color = (RgbColor 45 45 48),
        [string]$FontName = "Segoe UI",
        [bool]$Bold = $false,
        [int]$Align = 1,
        [int]$Vertical = 1
    )

    $shape = $Slide.Shapes.AddTextbox(1, $Left, $Top, $Width, $Height)
    $shape.TextFrame2.MarginLeft = 0
    $shape.TextFrame2.MarginRight = 0
    $shape.TextFrame2.MarginTop = 0
    $shape.TextFrame2.MarginBottom = 0
    $shape.TextFrame2.VerticalAnchor = $Vertical
    $shape.TextFrame2.TextRange.Text = $Text
    $shape.TextFrame2.TextRange.Font.Name = $FontName
    $shape.TextFrame2.TextRange.Font.Size = $FontSize
    $shape.TextFrame2.TextRange.Font.Fill.ForeColor.RGB = $Color
    if ($Bold) {
        $shape.TextFrame2.TextRange.Font.Bold = -1
    }
    $shape.TextFrame2.TextRange.ParagraphFormat.Alignment = $Align
    return $shape
}

function Add-Rect {
    param(
        [object]$Slide,
        [double]$Left,
        [double]$Top,
        [double]$Width,
        [double]$Height,
        [int]$Fill,
        [int]$Line = $null,
        [double]$Transparency = 0,
        [int]$ShapeType = 5
    )

    $shape = $Slide.Shapes.AddShape($ShapeType, $Left, $Top, $Width, $Height)
    $shape.Fill.ForeColor.RGB = $Fill
    $shape.Fill.Transparency = $Transparency
    if ($null -eq $Line) {
        $shape.Line.Visible = 0
    } else {
        $shape.Line.Visible = -1
        $shape.Line.ForeColor.RGB = $Line
        $shape.Line.Weight = 1.1
    }
    return $shape
}

function Add-Line {
    param(
        [object]$Slide,
        [double]$X1,
        [double]$Y1,
        [double]$X2,
        [double]$Y2,
        [int]$Color = (RgbColor 110 118 128),
        [double]$Weight = 1.5,
        [bool]$Arrow = $false
    )
    $line = $Slide.Shapes.AddLine($X1, $Y1, $X2, $Y2)
    $line.Line.ForeColor.RGB = $Color
    $line.Line.Weight = $Weight
    if ($Arrow) {
        $line.Line.EndArrowheadStyle = 3
    }
    return $line
}

function Add-Chip {
    param(
        [object]$Slide,
        [string]$Text,
        [double]$Left,
        [double]$Top,
        [double]$Width,
        [int]$Fill = (RgbColor 0 104 211),
        [int]$TextColor = (RgbColor 255 255 255)
    )
    Add-Rect $Slide $Left $Top $Width 24 $Fill $null 0 5 | Out-Null
    Add-TextBox $Slide $Text ($Left + 10) ($Top + 3) ($Width - 20) 17 9 $TextColor "Segoe UI Semibold" $false 2 3 | Out-Null
}

function Add-Card {
    param(
        [object]$Slide,
        [string]$Title,
        [string]$Body,
        [double]$Left,
        [double]$Top,
        [double]$Width,
        [double]$Height,
        [string]$Number = "",
        [int]$Fill = (RgbColor 245 246 248),
        [int]$Accent = (RgbColor 0 104 211),
        [bool]$Dark = $false
    )
    $textColor = if ($Dark) { RgbColor 255 255 255 } else { RgbColor 43 43 46 }
    $bodyColor = if ($Dark) { RgbColor 236 239 243 } else { RgbColor 91 94 99 }
    Add-Rect $Slide $Left $Top $Width $Height $Fill $null 0 5 | Out-Null
    Add-TextBox $Slide $Title ($Left + 22) ($Top + 22) ($Width - 44) 30 19 $textColor "Segoe UI Semibold" $false 1 1 | Out-Null
    Add-TextBox $Slide $Body ($Left + 22) ($Top + 62) ($Width - 44) ($Height - 84) 12 $bodyColor "Segoe UI" $false 1 1 | Out-Null
    if ($Number.Trim() -ne "") {
        Add-Rect $Slide ($Left + $Width - 54) ($Top + $Height - 54) 40 40 $Accent $null 0 9 | Out-Null
        Add-TextBox $Slide $Number ($Left + $Width - 54) ($Top + $Height - 47) 40 20 14 (RgbColor 255 255 255) "Segoe UI Semibold" $true 2 3 | Out-Null
    }
}

function Add-PictureFit {
    param(
        [object]$Slide,
        [string]$Path,
        [double]$Left,
        [double]$Top,
        [double]$Width,
        [double]$Height
    )
    Add-Type -AssemblyName System.Drawing
    $img = [System.Drawing.Image]::FromFile($Path)
    try {
        $ratio = $img.Width / $img.Height
    } finally {
        $img.Dispose()
    }
    $boxRatio = $Width / $Height
    if ($ratio -gt $boxRatio) {
        $newWidth = $Width
        $newHeight = $Width / $ratio
        $newLeft = $Left
        $newTop = $Top + (($Height - $newHeight) / 2)
    } else {
        $newHeight = $Height
        $newWidth = $Height * $ratio
        $newLeft = $Left + (($Width - $newWidth) / 2)
        $newTop = $Top
    }
    return $Slide.Shapes.AddPicture($Path, $false, $true, $newLeft, $newTop, $newWidth, $newHeight)
}

function Add-SlideNumber {
    param([object]$Slide, [int]$Number)
    Add-TextBox $Slide ([string]$Number) 925 512 20 12 9 (RgbColor 82 86 92) "Segoe UI" $false 2 1 | Out-Null
}

function Add-Header {
    param(
        [object]$Slide,
        [string]$Kicker,
        [string]$Title,
        [string]$Subtitle = "",
        [int]$Number = 0
    )
    Add-TextBox $Slide $Kicker 46 35 340 18 12 (RgbColor 66 69 75) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-TextBox $Slide $Title 46 62 560 70 31 (RgbColor 45 45 48) "Segoe UI Semibold" $true 1 1 | Out-Null
    if ($Subtitle.Trim() -ne "") {
        Add-TextBox $Slide $Subtitle 630 55 270 78 15 (RgbColor 85 88 95) "Segoe UI" $false 1 1 | Out-Null
    }
    if ($Number -gt 0) {
        Add-SlideNumber $Slide $Number
    }
}

function Add-BaseBackground {
    param([object]$Slide, [bool]$Dark = $false)
    if ($Dark) {
        Add-Rect $Slide 0 0 960 540 (RgbColor 46 47 49) $null 0 1 | Out-Null
    } else {
        Add-Rect $Slide 0 0 960 540 (RgbColor 239 240 242) $null 0 1 | Out-Null
        Add-Rect $Slide 0 0 960 540 (RgbColor 255 255 255) $null 0.74 1 | Out-Null
    }
}

function Add-ServiceBox {
    param(
        [object]$Slide,
        [string]$Title,
        [string]$Subtitle,
        [double]$Left,
        [double]$Top,
        [int]$Fill = (RgbColor 54 133 207)
    )
    Add-Rect $Slide $Left $Top 174 74 $Fill (RgbColor 33 104 178) 0 5 | Out-Null
    Add-TextBox $Slide $Title ($Left + 14) ($Top + 14) 146 22 17 (RgbColor 255 255 255) "Segoe UI Semibold" $true 2 1 | Out-Null
    Add-TextBox $Slide $Subtitle ($Left + 14) ($Top + 42) 146 20 9 (RgbColor 236 244 255) "Segoe UI" $false 2 1 | Out-Null
}

function Add-Metric {
    param(
        [object]$Slide,
        [string]$Value,
        [string]$Label,
        [double]$Left,
        [double]$Top,
        [double]$Width
    )
    Add-Rect $Slide $Left $Top $Width 92 (RgbColor 245 246 248) $null 0 5 | Out-Null
    Add-TextBox $Slide $Value ($Left + 16) ($Top + 16) ($Width - 32) 30 22 (RgbColor 0 96 190) "Segoe UI Semibold" $true 2 1 | Out-Null
    Add-TextBox $Slide $Label ($Left + 16) ($Top + 51) ($Width - 32) 25 10 (RgbColor 83 87 94) "Segoe UI" $false 2 1 | Out-Null
}

if ([string]::IsNullOrWhiteSpace($ScriptRootOverride)) {
    if (-not [string]::IsNullOrWhiteSpace($PSScriptRoot)) {
        $scriptDir = $PSScriptRoot
    } elseif ($MyInvocation.MyCommand.Path) {
        $scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
    } else {
        $scriptDir = (Get-Location).Path
    }
} else {
    $scriptDir = $ScriptRootOverride
}
if ([string]::IsNullOrWhiteSpace($OutputDir)) {
    $OutputDir = $scriptDir
}
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$OutputDir = Resolve-Path $OutputDir
$assetsDir = Join-Path $scriptDir "assets"
$logoPath = Join-Path $assetsDir "mai-logo-1.png"
$qrPath = Join-Path $repoRoot "diploma-text\repository_qr_code.png"
$driftDetectionDiagram = Join-Path $repoRoot "diploma-text\platform-arch-images\vkr-dynamic-drift-detection.png"
$pptxPath = Join-Path $OutputDir "Infrastructure-Management-Platform-defense.pptx"
$pdfPath = Join-Path $OutputDir "Infrastructure-Management-Platform-defense.pdf"
$previewDir = Join-Path $OutputDir "preview"

if (Test-Path $previewDir) {
    Remove-Item $previewDir -Recurse -Force
}
New-Item -ItemType Directory -Force $previewDir | Out-Null
foreach ($existingOutput in @($pptxPath, $pdfPath)) {
    if (Test-Path $existingOutput) {
        Remove-Item $existingOutput -Force
    }
}

$ppLayoutBlank = 12
$powerpoint = $null
$presentation = $null

try {
    $powerpoint = New-Object -ComObject PowerPoint.Application
    $powerpoint.Visible = [Microsoft.Office.Core.MsoTriState]::msoTrue
    $presentation = $powerpoint.Presentations.Add()
    $presentation.PageSetup.SlideWidth = 960
    $presentation.PageSetup.SlideHeight = 540

    # Slide 1
    $slide = $presentation.Slides.Add(1, $ppLayoutBlank)
    Add-BaseBackground $slide
    Add-Rect $slide 28 30 904 478 (RgbColor 232 233 236) (RgbColor 218 220 224) 0 5 | Out-Null
    if (Test-Path $logoPath) {
        Add-PictureFit $slide $logoPath 58 63 58 58 | Out-Null
    }
    Add-TextBox $slide "Московский`nавиационный`nинститут" 132 63 145 64 20 (RgbColor 32 34 38) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-Line $slide 292 65 292 125 (RgbColor 55 57 62) 1.3 | Out-Null
    Add-TextBox $slide "национальный`nисследовательский`nуниверситет" 310 64 190 60 19 (RgbColor 32 34 38) "Segoe UI" $false 1 1 | Out-Null
    Add-TextBox $slide "ВЫПУСКНАЯ КВАЛИФИКАЦИОННАЯ`nРАБОТА БАКАЛАВРА" 62 148 460 64 22 (RgbColor 25 26 28) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-TextBox $slide "Автоматизированная платформа управления инфраструктурой с поддержкой подхода Architecture-as-Code" 62 240 765 88 30 (RgbColor 48 49 53) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-TextBox $slide "Направление: 01.03.02 «Прикладная математика и информатика»`nПрофиль: Информатика • Группа М8О-409Б-22" 62 366 565 50 15 (RgbColor 48 49 53) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-TextBox $slide "Студент: Концебалов Олег Сергеевич`nРуководитель: Дзюба Дмитрий Владимирович`nКонсультант: Булакина Мария Борисовна" 590 405 310 62 16 (RgbColor 48 49 53) "Segoe UI" $false 1 1 | Out-Null
    Add-TextBox $slide "Москва • 2026" 62 455 150 20 12 (RgbColor 91 94 99) "Segoe UI" $false 1 1 | Out-Null

    # Slide 2
    $slide = $presentation.Slides.Add(2, $ppLayoutBlank)
    Add-BaseBackground $slide
    Add-Rect $slide 30 32 900 216 (RgbColor 47 48 51) $null 0 5 | Out-Null
    Add-TextBox $slide "ПОСТАНОВКА ЗАДАЧИ" 60 63 250 18 13 (RgbColor 244 245 247) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-TextBox $slide "Актуальность, цель и задача" 60 92 520 55 31 (RgbColor 255 255 255) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-TextBox $slide "Инфраструктура меняется быстрее документации: ручные правки, сбои и неполные rollout-процедуры приводят к drift между архитектурным описанием и фактической средой." 60 162 610 46 15 (RgbColor 235 237 241) "Segoe UI" $false 1 1 | Out-Null
    Add-Rect $slide 720 70 145 118 (RgbColor 62 135 246) $null 0 9 | Out-Null
    Add-TextBox $slide "desired`nstate`n≠`nactual`nstate" 739 88 106 78 19 (RgbColor 255 255 255) "Segoe UI Semibold" $true 2 3 | Out-Null
    Add-Card $slide "Архитектура" "Structurizr/C4-модель фиксирует требуемые host-level workload." 45 278 202 150 "01" (RgbColor 247 248 250) (RgbColor 0 104 211)
    Add-Card $slide "Фактическое состояние" "Inventory-сервис получает runtime-данные из cAdvisor и отмечает partial result." 268 278 202 150 "02" (RgbColor 247 248 250) (RgbColor 0 104 211)
    Add-Card $slide "Drift" "Подтвержденное расхождение при полной actual data." 491 278 202 150 "03" (RgbColor 247 248 250) (RgbColor 0 104 211)
    Add-Card $slide "Цель" "Разработать платформу, которая строит desired state, получает actual state, обнаруживает drift и запускает reconcile." 714 278 202 150 "04" (RgbColor 0 104 211) (RgbColor 47 48 51) $true
    Add-TextBox $slide "Вывод: работа переводит архитектурное описание из статичной документации в источник данных для control loop." 48 462 780 24 15 (RgbColor 45 45 48) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-SlideNumber $slide 2

    # Slide 3
    $slide = $presentation.Slides.Add(3, $ppLayoutBlank)
    Add-BaseBackground $slide
    Add-Header $slide "СРАВНЕНИЕ С АНАЛОГАМИ" "Существующие решения закрывают части задачи" "Для ВКР важен легковесный контур: Architecture-as-Code → inventory → drift detection → reconcile." 3
    $columns = @()
    $columns += ,@("Ansible", "agentless, зрелая экосистема", "job-based запуск без встроенной непрерывной реконсиляции")
    $columns += ,@("Terraform / Pulumi", "декларативное описание ресурсов", "фокус на provisioning и state-файл, не на host-level workload")
    $columns += ,@("K8s / GitOps", "сильная control-loop модель", "естественная область — Kubernetes API и кластерные ресурсы")
    $columns += ,@("Проект ВКР", "Architecture-as-Code как source of truth", "легковесный MVP для Docker-based workload вне Kubernetes")
    $x = 56
    foreach ($c in $columns) {
        $fill = if ($c[0] -eq "Проект ВКР") { RgbColor 0 104 211 } else { RgbColor 245 246 248 }
        $dark = $c[0] -eq "Проект ВКР"
        Add-Card $slide $c[0] ("Сильная сторона:`n" + $c[1] + "`n`nОграничение:`n" + $c[2]) $x 170 197 245 "" $fill (RgbColor 0 104 211) $dark
        $x += 220
    }
    Add-TextBox $slide "Вывод: собственная платформа нужна не вместо этих инструментов, а как связующий control plane для выбранного сценария." 58 450 820 30 15 (RgbColor 45 45 48) "Segoe UI Semibold" $true 1 1 | Out-Null

    # Slide 4
    $slide = $presentation.Slides.Add(4, $ppLayoutBlank)
    Add-BaseBackground $slide
    Add-Header $slide "ТРЕБОВАНИЯ" "Ключевые требования к системе" "Требования сформулированы вокруг безопасного цикла управления состоянием инфраструктуры." 4
    Add-Card $slide "Функциональные" "• загрузка Structurizr JSON`n• построение host-centric desired state`n• сбор actual state через cAdvisor`n• обнаружение отсутствующих workload`n• запуск reconcile для node_exporter и cadvisor" 55 160 258 250 "F" (RgbColor 245 246 248) (RgbColor 0 104 211)
    Add-Card $slide "Архитектурные" "• четыре независимых сервиса`n• явные HTTP API-контракты`n• read-only snapshot для state-сервисов`n• registry detector-ов и operator-ов`n• контейнеризация и внешний конфиг" 351 160 258 250 "A" (RgbColor 245 246 248) (RgbColor 0 104 211)
    Add-Card $slide "Эксплуатационные" "• partial data не считается drift`n• cooldown подавляет повторные команды`n• асинхронная очередь reconcile`n• ограничение параллелизма worker-ов`n• SSH-параметры через окружение" 647 160 258 250 "O" (RgbColor 47 48 51) (RgbColor 0 104 211) $true
    Add-TextBox $slide "Ключевое следствие: платформа автоматизирует только подтвержденные и заранее поддержанные действия восстановления." 58 448 820 30 15 (RgbColor 45 45 48) "Segoe UI Semibold" $true 1 1 | Out-Null

    # Slide 5
    $slide = $presentation.Slides.Add(5, $ppLayoutBlank)
    Add-BaseBackground $slide
    Add-Header $slide "АРХИТЕКТУРА" "Архитектура решения и стек технологий" "Упрощённая схема построена по C4-диаграммам проекта и реальным сервисным контрактам." 5
    Add-Rect $slide 46 156 575 276 (RgbColor 255 255 255) (RgbColor 221 225 230) 0 5 | Out-Null
    Add-ServiceBox $slide "ParserSvc" "Go, Gin, REST API" 244 185
    Add-ServiceBox $slide "InventorySvc" "Go, Gin, cAdvisor" 244 310
    Add-ServiceBox $slide "DriftDetectorSvc" "Go, scheduler, clients" 468 247 (RgbColor 0 104 211)
    Add-ServiceBox $slide "ReconcilerSvc" "Python, FastAPI, Ansible" 690 247 (RgbColor 54 60 67)
    Add-Rect $slide 56 185 160 86 (RgbColor 245 246 248) (RgbColor 28 31 35) 0 5 | Out-Null
    Add-TextBox $slide "AaC" 78 205 118 24 19 (RgbColor 45 45 48) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-TextBox $slide "Structurizr JSON`nманифесты" 78 237 118 30 10 (RgbColor 83 87 94) "Segoe UI" $false 1 1 | Out-Null
    Add-Rect $slide 56 310 160 86 (RgbColor 245 246 248) (RgbColor 28 31 35) 0 5 | Out-Null
    Add-TextBox $slide "Управляемый хост" 78 327 118 34 16 (RgbColor 45 45 48) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-TextBox $slide "Docker, cAdvisor, SSH" 78 365 118 17 10 (RgbColor 83 87 94) "Segoe UI" $false 1 1 | Out-Null
    Add-Line $slide 216 226 244 222 (RgbColor 110 118 128) 1.5 $true | Out-Null
    Add-Line $slide 216 351 244 347 (RgbColor 110 118 128) 1.5 $true | Out-Null
    Add-Line $slide 418 222 468 268 (RgbColor 110 118 128) 1.5 $true | Out-Null
    Add-Line $slide 418 347 468 294 (RgbColor 110 118 128) 1.5 $true | Out-Null
    Add-Line $slide 642 284 690 284 (RgbColor 110 118 128) 1.7 $true | Out-Null
    Add-Line $slide 777 321 777 402 (RgbColor 110 118 128) 1.4 | Out-Null
    Add-Line $slide 777 402 216 376 (RgbColor 110 118 128) 1.4 $true | Out-Null
    Add-TextBox $slide "desired state" 430 207 95 18 9 (RgbColor 91 94 99) "Segoe UI" $false 2 1 | Out-Null
    Add-TextBox $slide "actual state" 430 351 95 18 9 (RgbColor 91 94 99) "Segoe UI" $false 2 1 | Out-Null
    Add-TextBox $slide "reconcile" 640 258 70 16 9 (RgbColor 91 94 99) "Segoe UI" $false 2 1 | Out-Null
    Add-Rect $slide 644 158 261 74 (RgbColor 245 246 248) $null 0 5 | Out-Null
    Add-TextBox $slide "Стек MVP" 664 174 100 20 16 (RgbColor 45 45 48) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-Chip $slide "Go" 663 204 42
    Add-Chip $slide "FastAPI" 712 204 78 (RgbColor 54 60 67)
    Add-Chip $slide "Docker" 800 204 72 (RgbColor 54 60 67)
    Add-Chip $slide "Structurizr JSON" 644 425 126 (RgbColor 0 104 211)
    Add-Chip $slide "cAdvisor" 782 425 82 (RgbColor 54 60 67)
    Add-Chip $slide "Ansible Runner" 644 456 126 (RgbColor 54 60 67)
    Add-TextBox $slide "Сервисы не разделяют внутреннее состояние: каждый слой скрыт за API, что снижает связность и упрощает развитие." 56 462 520 30 13 (RgbColor 45 45 48) "Segoe UI Semibold" $true 1 1 | Out-Null

    # Slide 6
    $slide = $presentation.Slides.Add(6, $ppLayoutBlank)
    Add-BaseBackground $slide
    Add-Header $slide "МОДЕЛЬ И АЛГОРИТМ" "Control loop: desired → actual → drift → reconcile" "Вместо ML-модели в работе используется модель согласования состояния инфраструктуры." 6
    Add-Rect $slide 48 162 372 290 (RgbColor 47 48 51) $null 0 5 | Out-Null
    $steps = @()
    $steps += ,@("01", "Desired state", "ParserSvc извлекает host-centric модель из Structurizr JSON.")
    $steps += ,@("02", "Actual state", "InventorySvc формирует snapshot наблюдаемых workload.")
    $steps += ,@("03", "Drift detection", "Detector сравнивает состояния и учитывает partial data.")
    $steps += ,@("04", "Cooldown", "Повторные reconcile-команды подавляются в заданном окне.")
    $steps += ,@("05", "Reconcile", "ReconcilerSvc ставит задачу в очередь и запускает Ansible playbook.")
    $y = 182
    foreach ($s in $steps) {
        Add-Rect $slide 70 $y 40 40 (RgbColor 0 104 211) $null 0 9 | Out-Null
        Add-TextBox $slide $s[0] 70 ($y + 9) 40 18 12 (RgbColor 255 255 255) "Segoe UI Semibold" $true 2 1 | Out-Null
        Add-TextBox $slide $s[1] 126 ($y + 1) 220 20 14 (RgbColor 255 255 255) "Segoe UI Semibold" $true 1 1 | Out-Null
        Add-TextBox $slide $s[2] 126 ($y + 23) 250 20 9 (RgbColor 228 232 238) "Segoe UI" $false 1 1 | Out-Null
        if ($y -lt 370) {
            Add-Line $slide 90 ($y + 42) 90 ($y + 56) (RgbColor 145 188 255) 1.2 $true | Out-Null
        }
        $y += 52
    }
    Add-Rect $slide 456 162 430 290 (RgbColor 255 255 255) (RgbColor 221 225 230) 0 5 | Out-Null
    Add-PictureFit $slide $driftDetectionDiagram 474 176 394 252 | Out-Null
    Add-TextBox $slide "Реальная динамическая диаграмма сценария обнаружения drift из материалов проекта" 474 427 390 17 9 (RgbColor 91 94 99) "Segoe UI" $false 2 1 | Out-Null
    Add-TextBox $slide "Ключевая гарантия MVP: отсутствие данных не является основанием для автоматического восстановления." 58 474 820 22 15 (RgbColor 45 45 48) "Segoe UI Semibold" $true 1 1 | Out-Null

    # Slide 7
    $slide = $presentation.Slides.Add(7, $ppLayoutBlank)
    Add-BaseBackground $slide
    Add-Header $slide "ТЕСТИРОВАНИЕ И МЕТРИКИ" "Проверка качества разработанного решения" "Использованы unit/component tests и E2E benchmark harness поверх локального compose-стенда." 7
    Add-Rect $slide 50 162 365 240 (RgbColor 255 255 255) (RgbColor 221 225 230) 0 5 | Out-Null
    Add-TextBox $slide "Detection scalability, p50" 75 183 250 20 15 (RgbColor 45 45 48) "Segoe UI Semibold" $true 1 1 | Out-Null
    $bars = @()
    $bars += ,@("1", 0.0, 0)
    $bars += ,@("10", 1.0, 10)
    $bars += ,@("100", 6.0, 62)
    $bars += ,@("500", 20.0, 205)
    $barY = 224
    foreach ($b in $bars) {
        Add-TextBox $slide ($b[0] + " hosts") 75 ($barY + 2) 70 14 9 (RgbColor 83 87 94) "Segoe UI" $false 1 1 | Out-Null
        Add-Rect $slide 150 $barY ([Math]::Max(4, $b[2])) 16 (RgbColor 0 104 211) $null 0 5 | Out-Null
        Add-TextBox $slide ([string]$b[1] + " ms") (160 + [Math]::Max(4, $b[2])) ($barY + 1) 55 13 9 (RgbColor 83 87 94) "Segoe UI" $false 1 1 | Out-Null
        $barY += 38
    }
    Add-TextBox $slide "Рост цикла в steady-state связан в основном с fetch-стадиями Inventory и Parser." 75 367 290 24 10 (RgbColor 83 87 94) "Segoe UI" $false 1 1 | Out-Null
    Add-Metric $slide "14.3 s" "сходимость после drift node_exporter" 455 162 130
    Add-Metric $slide "102 / 180" "accepted при saturation probe" 610 162 160
    Add-Metric $slide "1.0" "cooldown suppression ratio" 795 162 110
    Add-Card $slide "Покрытие тестами" "Unit/component tests по четырем сервисам.`nE2E harness: 5 benchmark-сценариев." 455 285 206 142 "" (RgbColor 245 246 248) (RgbColor 0 104 211)
    Add-Card $slide "Partial data" "20 host, 6 недоступных FQDN.`ncycle.partial=true, failedHosts=6, reconcile для missing hosts=0." 685 285 220 142 "" (RgbColor 47 48 51) (RgbColor 0 104 211) $true
    Add-TextBox $slide "Вывод: MVP подтверждает работоспособность control loop и явно показывает границы производительности и безопасности." 58 462 830 26 15 (RgbColor 45 45 48) "Segoe UI Semibold" $true 1 1 | Out-Null

    # Slide 8
    $slide = $presentation.Slides.Add(8, $ppLayoutBlank)
    Add-BaseBackground $slide
    Add-Rect $slide 30 32 900 158 (RgbColor 0 39 96) $null 0 5 | Out-Null
    Add-TextBox $slide "РЕЗУЛЬТАТЫ РАБОТЫ" 60 60 220 18 13 (RgbColor 255 255 255) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-TextBox $slide "Результаты разработки MVP" 60 90 620 42 30 (RgbColor 255 255 255) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-TextBox $slide "Архитектурное описание, inventory, drift detection и reconcile связаны в единый программный цикл." 60 140 650 24 14 (RgbColor 226 236 255) "Segoe UI" $false 1 1 | Out-Null
    Add-Card $slide "ParserSvc" "desired state`nStructurizr JSON" 55 225 190 92 "" (RgbColor 245 246 248) (RgbColor 0 104 211)
    Add-Card $slide "InventorySvc" "actual state`npartial metadata" 268 225 190 92 "" (RgbColor 245 246 248) (RgbColor 0 104 211)
    Add-Card $slide "DriftDetectorSvc" "detectors`ncooldown" 55 342 190 92 "" (RgbColor 245 246 248) (RgbColor 0 104 211)
    Add-Card $slide "ReconcilerSvc" "operators`nAnsible Runner" 268 342 190 92 "" (RgbColor 245 246 248) (RgbColor 0 104 211)
    Add-Rect $slide 515 225 175 175 (RgbColor 255 255 255) (RgbColor 221 225 230) 0 1 | Out-Null
    Add-PictureFit $slide $qrPath 530 240 145 145 | Out-Null
    Add-TextBox $slide "QR на GitHub" 515 405 175 17 11 (RgbColor 45 45 48) "Segoe UI Semibold" $true 2 1 | Out-Null
    Add-TextBox $slide "github.com/BaronPipistron/Infrastructure-Management-Platform" 475 435 255 28 9 (RgbColor 83 87 94) "Segoe UI" $false 2 1 | Out-Null
    Add-Card $slide "Практический результат" "• Docker Compose E2E`n• node_exporter и cadvisor`n• benchmark harness`n• ограничения MVP зафиксированы" 740 225 170 210 "" (RgbColor 47 48 51) (RgbColor 0 104 211) $true
    Add-SlideNumber $slide 8

    # Slide 9
    $slide = $presentation.Slides.Add(9, $ppLayoutBlank)
    Add-BaseBackground $slide $true
    Add-Rect $slide 30 32 900 220 (RgbColor 62 135 246) $null 0 5 | Out-Null
    Add-TextBox $slide "ИНЖЕНЕРНАЯ ЦЕННОСТЬ" 60 60 250 18 13 (RgbColor 255 255 255) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-TextBox $slide "Architecture-as-Code в эксплуатационном контуре" 60 94 720 48 30 (RgbColor 255 255 255) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-TextBox $slide "Результат ВКР — проверенная декомпозиция будущего internal control plane для host-level workload." 60 156 705 36 15 (RgbColor 239 246 255) "Segoe UI" $false 1 1 | Out-Null
    Add-Card $slide "Ценность для команды" "• меньше ручных сверок`n• архитектурная модель используется машинно`n• drift обнаруживается до инцидента`n• восстановление через operator-ы" 58 288 250 162 "" (RgbColor 245 246 248) (RgbColor 0 104 211)
    Add-Card $slide "Расширяемость" "• новые actual-state sources: NetBox, CMDB, Prometheus`n• новые detector-ы и operator-ы`n• version/config/health drift`n• persistence истории и job state" 355 288 250 162 "" (RgbColor 245 246 248) (RgbColor 0 104 211)
    Add-Card $slide "Границы MVP" "• in-memory state`n• два workload`n• нет authN/authZ и audit trail`n• нет API статуса reconcile-задачи`n• результаты benchmark не являются SLA" 652 288 250 162 "" (RgbColor 245 246 248) (RgbColor 0 104 211)
    Add-TextBox $slide "Итог: цель ВКР достигнута — реализован и проверен базовый цикл desired state → actual state → drift → reconcile." 58 486 780 22 15 (RgbColor 255 255 255) "Segoe UI Semibold" $true 1 1 | Out-Null
    Add-SlideNumber $slide 9

    $presentation.SaveAs($pptxPath)
    $presentation.SaveAs($pdfPath, 32)
    $presentation.Export($previewDir, "PNG", 1600, 900)
} finally {
    if ($null -ne $presentation) {
        $presentation.Close()
    }
    if ($null -ne $powerpoint) {
        $powerpoint.Quit()
    }
    if ($null -ne $presentation) {
        [System.Runtime.InteropServices.Marshal]::ReleaseComObject($presentation) | Out-Null
    }
    if ($null -ne $powerpoint) {
        [System.Runtime.InteropServices.Marshal]::ReleaseComObject($powerpoint) | Out-Null
    }
    [GC]::Collect()
    [GC]::WaitForPendingFinalizers()
}

Write-Host "Created: $pptxPath"
Write-Host "Created: $pdfPath"
Write-Host "Preview: $previewDir"
