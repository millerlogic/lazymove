[![GoDoc](https://godoc.org/github.com/millerlogic/lazymove?status.svg)](https://godoc.org/github.com/millerlogic/lazymove)

# lazymove
Lazily move files from one directory to another.
This is useful when you have slower long-term storage but want to be able to write new files quickly.
You can write to the fast disk, and use lazymove to lazily and asynchronously move the files to the slower storage, such as a network mount.

See [godoc](https://godoc.org/github.com/millerlogic/lazymove) for API usage, or use the command:

```
Usage: ./lazymove [Options...] <SourceDir> <DestDir>
Options:
  -MinDirAge duration
    	Minimum age to remove empty dirs (default 1h0m0s)
  -MinFileAge duration
    	Minimum age to move files (default 5m0s)
  -Timeout duration
    	How often to look for files to move (default 5m0s)
```

The mover will lazily move files from SourceDir into DestDir. It will do this iteration each Timeout, only moving (copy, delete) files from SourceDir to DestDir with modification times older than MinFileAge, and only removing empty directories from SourceDir with times older than MinDirAge.
