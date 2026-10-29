# Documentation

## Linting and formatting

The Taskfile implements:

- `task setup` - install the `mdox` formatter for markdown (uses `go install`)
- `task fmt` - format markdown files according to mdox rules, remove soft wraps
- `task lint` - use [vale](https://vale.sh) for the `llm-slop` style and glossary checks
- `task up` - bring up the environment for editing with `docker compose up -d`
- `task down` - shut down the compose environment after done editing

vale reads its rules from `.vale.ini` and `.vale/`: the `llm-slop` style from [phpscript](https://github.com/titpetric/phpscript), and the `tech` vocabulary of accepted terms.

Sometimes, I write like [The Grug Brained Developer](https://grugbrain.dev/) and it would be a good check that my textual outputs resemble some sort of formal English language structure.

> Removing soft wrapping is a design choice. For changes in markdown files, GitHub shows a rich-diff view. Editors differ in screen size, and resizing the window makes the content more readable. Blog systems like [gohugo.io](https://gohugo.io) ignore soft line wrapping anyway. We have a hard time reviewing source code, let alone invisible space.
>
> Move your experience to a basic editor if that's a problem.

## Editing

The `docker-compose.yml` file starts an instance of [dullage/flatnotes](https://github.com/dullage/flatnotes). It starts flatnotes with authentication off. Open [http://127.0.0.1:8081](http://localhost:8081) to log in. To host the editor for a team, turn on authentication in flatnotes.

It supports image uploads.

![pengu-pudgy.gif](attachments/pengu-pudgy.gif)

A folder structure option would be nice.
