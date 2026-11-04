// Adapted from VidOtter's successful-installation hooks; see third_party/vidotter.
Component.prototype.showFirstRunOptions = function() {
    if (installer.isInstaller() && !installer.isCommandLineInstance() &&
        installer.status == QInstaller.Success) {
        installer.addWizardPageItem(component, "WardrobeFirstRun", QInstaller.InstallationFinished);
    }
};
Component.prototype.finishFirstRun = function() {
    if (!installer.isInstaller() || installer.isCommandLineInstance() ||
        installer.status != QInstaller.Success) return;
    var form = component.userInterface("WardrobeFirstRun");
    if (!form) return;
    var target = installer.value("TargetDir");
    console.log("WARDROBE_FINISH_OPTIONS");
    if ("__PLATFORM__" == "windows") {
        var powershell = installer.environmentVariable("SystemRoot") + "/System32/WindowsPowerShell/v1.0/powershell.exe";
        if (form.openAppCheckBox.checked) {
            installer.executeDetached(powershell, ["-NoProfile", "-ExecutionPolicy", "Bypass",
                "-WindowStyle", "Hidden", "-File", installer.toNativeSeparators(target + "/scripts/Open-Wardrobe.ps1")], target);
        }
        if (form.openExtensionCheckBox.checked) {
            installer.executeDetached(powershell, ["-NoProfile", "-ExecutionPolicy", "Bypass",
                "-WindowStyle", "Hidden", "-File", installer.toNativeSeparators(target + "/scripts/Guide-Wardrobe-Extension.ps1")], target);
        }
    } else {
        if (form.openAppCheckBox.checked) {
            installer.executeDetached("/bin/bash", [target + "/scripts/Open-Wardrobe.sh"], target);
        }
        if (form.openExtensionCheckBox.checked) {
            QDesktopServices.openUrl("file://" + target + "/browser-extension");
            QDesktopServices.openUrl("file://" + target + "/README.md");
        }
    }
};
