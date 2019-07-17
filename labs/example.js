window.addEventListener("load", function() {

    var btn = this.document.getElementById("testButton")

    btn.addEventListener("click", function(e) {
        var content = document.getElementById("content")
        
        if (btn.innerHTML == "Hide") {
            btn.innerHTML = "Show"
            content.className = "makeItDisappear"
            
            setTimeout(function() {
                content.style.display = "none"
            }, 1000);
        } else {
            btn.innerHTML = "Hide";
            content.style.display = "block"

            setTimeout(function () {
                content.className = "makeItNormal"
            }, 500);
        }
    });

    var img = this.document.getElementById("mainImage");

    img.addEventListener("mouseover", function(event) {
        img.className = "makeItGray";
    });

    img.addEventListener("mouseout", function (event) {
        img.className = "makeItNormal"
    })
});