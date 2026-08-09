"use strict";
try {
    if (window.localStorage.getItem("caroline-theme") === "light")
        document.documentElement.dataset.theme = "light";
}
catch {
    // Local storage can be unavailable before the application initializes.
}
