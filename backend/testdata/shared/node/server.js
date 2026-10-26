const http = require("http");
const port = process.env.PORT || 3000;
http.createServer((q, r) => r.end("ok")).listen(port);
