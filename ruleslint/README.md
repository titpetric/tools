# ruleslint

The intent of `ruleslint` is to lint a source tree for the accepted structure.

## Structure and config

```
ruleslint {
  name: TaskUI php app structure

  dir {
    modules
    theme
  }

  dir modules {
    login/
    threads/
    refinement/
    ...
  }

  dir theme {
    dao/
    templates/
    templates_c/
    index.php
  }

  dir modules/{module} {
    dao/
    templates/
    templates_c/
    schema/*.up.sql

    api/*.php {
      require: '@route'
      deny: 'class '
    }

    modinfo.php {
      require: 'new Task\Module'
    }

    index.php {
      require: 'class mod_{module} {'
      require: 'function get*'
    }

    mod_{submodule}.php {
      require: 'class mod_{submodule} {'
    }
  }

  dir modules/{module}/dao {
    index.php {
      require: 'class {module}_dao {'
    }
  }

  linters {
    phpscript lint ./...
  }
}
```

The `...` are not tool functionality, they are user input for checking.
Parsing is custom for the above schema. The parser is line based, blocks
are opened and closed with `{}` braces and evaluate to `[]model.Check` for
each line or brached statement.

A block under a path takes two keys. `require:` names text every file the
path matched holds, and `deny:` names text none of them may: a file of
`api/` routes and declares nothing, so it requires `@route` and denies
`class `.

## Install and use

You can install ruleslint with Go:

```
go install github.com/titpetric/tools/ruleslint@main
```

- Create a `.ruleslint.conf'
- Run `ruleslint` to validate
