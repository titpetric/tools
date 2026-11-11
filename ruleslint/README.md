# ruleslint

Check the directories of a site against a rule file.

```
ruleslint [-v] [rulefile]
```

`-v` lists every check of every module. See `ruleslint --help`. The rule file defaults to `rules/module.yml`; its
input globs resolve from the parent of the `rules/` directory.
