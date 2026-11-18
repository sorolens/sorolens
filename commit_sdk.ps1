$statusLines = git status --porcelain
$files = @()
foreach ($line in $statusLines) {
    if ($line.Length -gt 3) {
        $path = $line.Substring(3).Trim()
        if ($path -ne "commit_sdk.ps1") {
            $files += $path
        }
    }
}

Write-Host "Total files to commit: $($files.Count)"

foreach ($file in $files) {
    $baseName = Split-Path $file -Leaf
    $msg = "feat(python-sdk): sync generated $baseName with openapi spec"
    if ($msg.Length -gt 72) {
        $msg = "feat(python-sdk): sync generated $baseName"
    }

    Write-Host "Adding $file..."
    git add $file
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Failed to add $file"
        exit 1
    }

    Write-Host "Committing: $msg..."
    git commit -m $msg
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Failed to commit $file"
        exit 1
    }
}

Write-Host "All generated files committed individually!"
Write-Host "Pushing to remote origin feat/wave-enhancements..."
git push origin feat/wave-enhancements
if ($LASTEXITCODE -ne 0) {
    Write-Error "Failed to push"
    exit 1
}
Write-Host "Push completed successfully!"
