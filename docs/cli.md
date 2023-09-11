# CLI

Operates on directories as a unit of work:

```
my-model/
  substratus.yaml <---- Reads Dataset/Model/Server/Notebook from here.
  Dockerfile <--- Optional
  run.ipynb
```

## Alternative Names

### "ai" CLI

Pros

* Short and sweet

Cons

* Generic

```bash
# Looks for "ai.yaml"...

ai notebook .
ai nb .
ai run .
```

### "strat" CLI

Pros

* More specific than `sub`

Cons

* Longer than `sub`

```bash
strat notebook .
strat nb .
strat run .
```

## Notebook

```bash
sub notebook .
sub nb .
```

## Get

```bash
sub get
```

```
sub get

models/
  facebook-opt-125m
  falcon-7b

datasets/
  squad

servers/
  falcon-7b
```

```
sub get models

facebook-opt-125m
falcon-7b
```

```
sub get models/falcon-7b

v3
v2
v1
```

```
sub get models/falcon-7b.v3

metrics:
  loss: 0.9
  abc: 123
```

## Apply

```
# Alternative names???

# "run" --> currently prefer "apply" b/c the target is a noun/end-object (Model / Dataset / Server)
sub run .
```

### Apply (with `<dir>` arg)

* Tar & upload
* Remote build
* Wait for Ready

```bash
sub apply .
```

## View

* Grab `run.html` (converted notebook) and serve on localhost.
* Open browser.

```bash
sub view model/falcon-7b
sub view dataset/squad
```

Alternative names:

```bash
sub logs
sub show
sub inspect
```

## Delete

```bash
sub delete <resource>/<name>
sub del

# By name
sub delete models/facebook-opt-125m
sub delete datasets/squad
```

## Inference Client

```bash
sub infer
sub inf 

# OR:
sub cl (client)
sub ch (chat)     # LLM prompt/completion
sub cl (classify) # Image recognition
```

https://github.com/charmbracelet/bubbletea/tree/master/examples#chat

