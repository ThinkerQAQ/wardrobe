// Bind an explicit sandbox root: Qt --root applies to CLI installs only.
var ProductController = Controller;
Controller = function() {
    ProductController.call(this);
    installer.setValue("TargetDir", installer.value("WardrobeQARoot"));
    installer.installationFinished.connect(function() {
        gui.clickButton(buttons.NextButton, 800);
    });
};
Controller.prototype = ProductController.prototype;
// Append to the product controller for native Qt GUI regression.
var productIntroduction = Controller.prototype.IntroductionPageCallback;
Controller.prototype.IntroductionPageCallback = function() {
    productIntroduction.call(this);
    var message = gui.currentPageWidget().MessageLabel.text;
    console.log(message.indexOf("Install Wardrobe") >= 0 ? "QA_INTRO" : "QA_BAD_INTRO");
    gui.clickButton(buttons.NextButton, 1200);
};
Controller.prototype.TargetDirectoryPageCallback = function() {
    console.log("QA_TARGET");
    gui.clickButton(buttons.NextButton, 1200);
};
Controller.prototype.ComponentSelectionPageCallback = function() {
    console.log("QA_BAD_COMPONENT");
    gui.clickButton(buttons.NextButton, 1200);
};
Controller.prototype.StartMenuDirectoryPageCallback = function() {
    console.log("QA_BAD_STARTMENU");
    gui.clickButton(buttons.NextButton, 1200);
};
Controller.prototype.LicenseAgreementPageCallback = function() {
    console.log("QA_LICENSE");
    gui.currentPageWidget().AcceptLicenseCheckBox.checked = true;
    gui.clickButton(buttons.NextButton, 1200);
};
Controller.prototype.ReadyForInstallationPageCallback = function() {
    console.log("QA_READY");
    gui.clickButton(buttons.CommitButton, 1200);
};
Controller.prototype.FinishedPageCallback = function() {
    console.log("QA_FINISHED");
    var page=gui.currentPageWidget();
    var app=gui.findChild(page,"openAppCheckBox");
    var extension=gui.findChild(page,"openExtensionCheckBox");
    if (app && extension && app.checked && !extension.checked) console.log("QA_FINISH_OPTIONS");
    else console.log("QA_BAD_FINISH_OPTIONS");
    if(app) app.checked=false;
    if(extension) extension.checked=false;
    gui.clickButton(buttons.FinishButton, 1600);
};
