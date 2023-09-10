# CLI

Operates on directories as a unit of work:

```
my-model/
  substratus.yaml <---- Reads Dataset/Model/Server/Notebook from here.
  Dockerfile <--- Optional
  run.ipynb
```

## Alternative Names

"AI CLI":

```bash
# Looks for "ai.yaml"...

ai notebook .
ai nb .
ai run .
```

## Notebook

```bash
strat notebook .
strat nb .
```

## List

```bash
strat list
strat ls
```

```
strat ls

models/
  facebook-opt-125m
  falcon-7b

datasets/
  squad

servers/
  falcon-7b
```

```
strat ls models

facebook-opt-125m
falcon-7b
```

```
strat ls models/falcon-7b

model.bin
vocab.json
```

## Run

* Tar & upload
* Remote build
* Wait for Ready

```bash
strat run .
```

## View

* Grab `run.html` (converted notebook) and serve on localhost.
* Open browser.

```bash
strat view model/falcon-7b
strat view dataset/squad
```

Alternative names:

```bash
strat logs
strat show
strat inspect
```

## Delete

```bash
strat delete <resource>/<name>
strat del

# By name
strat delete models/facebook-opt-125m
strat delete datasets/squad
```

## Inference Client

```bash
strat infer
strat inf 

# OR:
strat cl (client)
strat ch (chat)     # LLM prompt/completion
strat cl (classify) # Image recognition
```

https://github.com/charmbracelet/bubbletea/tree/master/examples#chat

