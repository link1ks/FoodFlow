param(
    [string]$Python = 'python',
    [switch]$CheckOnly
)

$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$environmentPath = Join-Path $repoRoot '.cache\python-agent-venv'
$environmentPython = Join-Path $environmentPath 'Scripts\python.exe'

# No application configuration or credentials are read.
& $Python -c 'import sys, venv, ensurepip; print(sys.version); print(sys.executable); sys.exit(0 if sys.version_info[:2] == (3, 13) else 1)'
if ($LASTEXITCODE -ne 0) {
    throw 'Python 3.13 with venv and ensurepip is required.'
}
if ($CheckOnly) {
    Write-Output 'Prerequisites available. Check-only mode did not create an environment.'
    return
}

if (Test-Path -LiteralPath $environmentPath) {
    if (-not (Test-Path -LiteralPath $environmentPython)) {
        throw 'An incomplete environment already exists. Inspect it manually; this script never deletes it.'
    }
    Write-Output 'Using the existing project environment.'
} else {
    & $Python -m venv $environmentPath
    if ($LASTEXITCODE -ne 0) {
        throw 'Environment creation failed. Keep the output for diagnosis.'
    }
}

& $environmentPython -c "import sys; print('Project Python:', sys.executable); print('Version:', sys.version.split()[0]); sys.exit(0 if sys.prefix != sys.base_prefix and sys.version_info[:2] == (3, 13) else 1)"
if ($LASTEXITCODE -ne 0) {
    throw 'The project interpreter is not an isolated Python 3.13 environment.'
}
& $environmentPython -m pip --version
if ($LASTEXITCODE -ne 0) {
    throw 'pip is unavailable in the project environment.'
}
& $environmentPython -m pip install -r (Join-Path $repoRoot 'requirements-agent.txt')
if ($LASTEXITCODE -ne 0) { throw 'Pinned agent dependency installation failed.' }
& $environmentPython -m pip check
if ($LASTEXITCODE -ne 0) { throw 'Agent dependency compatibility check failed.' }
Write-Output 'LangChain/LangGraph agent environment ready. No provider requests, database access or deployment changes were made.'
