var darkSwitch = document.getElementById("darkSwitch");
window.addEventListener("load", function () {
    if (darkSwitch) {
        darkSwitch.checked = document.all[0].getAttribute("data-theme") === "dark";
        darkSwitch.addEventListener("change", function () {
            switchTheme();
        });
    }
});

function switchTheme() {
    if (darkSwitch.checked) {
        document.all[0].setAttribute("data-theme", "dark");
        localStorage.setItem("darkSwitch", "dark");
		console.log("Dark mode toggled on")
    } else {
        document.all[0].removeAttribute("data-theme");
        localStorage.setItem("darkSwitch", "light");
		console.log("Dark mode toggled off")
    }
}
