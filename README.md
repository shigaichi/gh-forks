# GitHub CLI Extension: gh-forks

This GitHub CLI extension allows you to retrieve and display the forks of a specified repository. You can navigate
through paginated results and sort them by different criteria such as stars and last updated time. The extension also
enables you to open forked repositories in your browser directly from the command line.

## Features

- Fetch and display forks of a GitHub repository
- Sort forks by stars or last updated time
- Paginated navigation for large datasets
- Open fork repositories in the browser
- Support for automatic detection of the repository from the current directory

## Installation

To install this extension for GitHub CLI (`gh`), run:

```sh
$ gh extension install https://github.com/shigaichi/gh-forks
```

## Usage

### Retrieve and Display Forks

To list the forks of a specific repository, use:

```sh
$ gh forks owner/repo
```

If you are in a local GitHub repository, you can simply run:

```sh
$ gh forks
```

### Navigation

Use the following keys to navigate through the forks:

- **↑ / k**: Move up
- **↓ / j**: Move down
- **← / h**: Previous page
- **→ / l**: Next page
- **s**: Sort by stars
- **u**: Sort by last updated
- **Enter**: Open the selected fork in the browser
- **q**: Quit

### Example Output

```
 GitHub Forks (Page 1, Sort: UPDATED_AT)  Total Forks: 656
 [↑/k] Up  [↓/j] Down  [←/h] Prev Page  [→/l] Next Page  [s] Sort by Stars  [u] Sort by Updated  [Enter] Open Repo  [q] Quit
Repo                              Stars   Ahead   Behind   Updated    Forks
user1/fork1                       102     5       10       2025-02-17  3
user2/fork2                        87     0        2       2025-02-16  1
...
```

## Requirements

- `gh` (GitHub CLI) installed
- A valid GitHub authentication setup
