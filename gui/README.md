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

## Kubectl proxy
When this GUI is used in combination with kubectl proxy, run the following proxy to deal with CORS:
```
npm install -g corsproxy
corsproxy
```
