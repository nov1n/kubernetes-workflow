# Flower GUI

## How to use
Start a simple python webserver from this folder:
```
cd gui
```

```
# Python 2.x
python -m SimpleHTTPServer
```

```
# Python 3.x
python -m http.server
```

Point your browser to
`http://localhost:8000/index.html#[namespace]/[wf-name]`

Substitute:
* `[namespace]` for the wf's namespace.
* `[wf-name]` for the wf's name.

## CORS proxy
To make the GUI work with kubectl proxy, we need another proxy to deal with CORS.
Run the following command to start the CORS proxy.
```
npm install -g corsproxy
corsproxy
```
