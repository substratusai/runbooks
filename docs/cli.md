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
strat nb (notebook)
```

## List

```bash
strat ls (list)
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

## Inference Client

```bash
strat in (interact)

# OR:
strat cl (client)
strat ch (chat)     # LLM prompt/completion
strat cl (classify) # Image recognition
```

https://github.com/charmbracelet/bubbletea/tree/master/examples#chat

