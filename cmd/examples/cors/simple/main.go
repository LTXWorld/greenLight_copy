package main

import (
	"flag"
	"log"
	"net/http"
)

// 使用fetch对我们的API healthcheck发送了一个请求，成功和失败都会对output标签进行修改并转储在response中
const html = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
</head>
<body>
	<h1>Simple CORS</h1>
	<div id="output"></div>
	<script>
		document.addEventListener('DOMContentLoaded', function() {
			fetch("http://localhost:4066/v1/healthcheck").then(
				function (response) {
					response.text().then(function (text) {
						document.getElementById("output").innerHTML = text;
					});
				},
				function(err) {
					document.getElementById("output").innerHTML = err;
				}
			);
		});
	</script>
</body>
</html>`

func main() {
	addr := flag.String("addr", ":9000", "Server address")
	flag.Parse()

	log.Printf("starting server on %s", *addr)

	err := http.ListenAndServe(*addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(html))
	}))
	log.Fatal(err)
}
