# build.ps1

# 1. Set Error Action Preference
# This will stop the script if any command fails.
$ErrorActionPreference = 'Continue'

# 2. Get Version
# Get the version from git describe.
$version = git describe --tags --always --dirty="-dev" --abbrev=7
Write-Host "Building version: $version"

# 3. Define ldflags
# Set the linker flags to embed the version in the binary.
$ldflags = "github.com/highercomve/tasktracker/internal/version.Version=$version"

# 4. Define platforms to build for
$platforms = @(
    @{os="windows"; arch="amd64"},
    @{os="linux"; arch="amd64"},
    @{os="linux"; arch="arm64"}
)

# 5. Loop through platforms and build
foreach ($platform in $platforms) {
    $os = $platform.os
    $arch = $platform.arch
    $archiveTarget = "$os-$arch"

    if ($os -eq "windows") {
        $appName = "tasktracker.exe"
    } else {
        $appName = "tasktracker"
    }

    Write-Host "Building for $archiveTarget..."
    
    try {
        # Build the command string exactly as confirmed to work, with quotes around ldflags.
        $command = "fyne-cross $($os) -arch=$($arch) -name $($appName) -icon Icon.png --app-id com.highercomve.tasktracker -ldflags=`"$($ldflags)`" ./cmd/tasktracker"
        
        Write-Host "Executing: $command"
        Invoke-Expression $command

        if ($LASTEXITCODE -ne 0) {
            throw "fyne-cross build failed with exit code $LASTEXITCODE"
        }

        Write-Host "Packaging for $archiveTarget..."

        if ($os -eq "windows") {
            # For Windows, fyne-cross generates the zip, just move it.
            $fyneCrossGeneratedArchive = "fyne-cross/dist/$os-$arch/$appName.zip"
            $finalArchiveName = "tasktracker-$archiveTarget.zip"

            if (-not (Test-Path -Path $fyneCrossGeneratedArchive)) {
                Write-Error "Fyne-cross generated archive not found: $fyneCrossGeneratedArchive"
                continue
            }
            Write-Host "Moving and renaming archive to $finalArchiveName..."
            Move-Item -Path $fyneCrossGeneratedArchive -Destination $finalArchiveName -Force
            Write-Host "Created archive: $finalArchiveName"
        } else {
            # For Linux, create tar.xz from the executable in fyne-cross/bin
            $binSourceDir = "fyne-cross/bin/$os-$arch"
            $executablePath = "$binSourceDir/$appName"
            $finalArchiveName = "tasktracker-$archiveTarget.tar.xz"

            if (-not (Test-Path -Path $executablePath)) {
                Write-Error "Executable not found for tar.xz: $executablePath"
                continue
            }
            Write-Host "Creating tar.xz for $archiveTarget..."
            tar -cJf $finalArchiveName -C $binSourceDir $appName
            Write-Host "Created archive: $finalArchiveName"
        }    } catch {
        Write-Error "Build failed for $archiveTarget. See error details below."
        Write-Error $_.Exception.Message
    }
}

Write-Host "All builds completed."
$ErrorActionPreference = 'Stop'

