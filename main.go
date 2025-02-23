package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbletea"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
	graphql "github.com/shurcooL/githubv4"
	"github.com/spf13/pflag"
)

type Fork struct {
	NameWithOwner  string
	StargazerCount int
	ForkCount      int
	UpdatedAt      time.Time
	Url            string
	AheadBy        int
	BehindBy       int
}

type model struct {
	forks      []Fork
	pages      map[int][]Fork
	page       int
	sortBy     string
	totalCount int
	loading    bool
	spinner    spinner.Model
	table      table.Model
	owner      string
	name       string
	headRef    string
	endCursor  *string
	lastPage   int
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchForks(m.owner, m.name, m.headRef, m.page, m.sortBy, m.endCursor))
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "down", "j":
			if m.table.Cursor() == m.table.Height()-1 && !m.loading && m.endCursor != nil && m.lastPage != m.page {
				return m.gotoNextPage()
			}
		case "up", "k":
			if m.table.Cursor() == 0 && m.page > 0 {
				m.gotoPreviousPage()
				m.table.GotoBottom()
				return m, nil
			}
		case "left", "h":
			if m.page > 0 && !m.loading {
				return m.gotoPreviousPage()
			}
		case "right", "l":
			if !m.loading && m.endCursor != nil && m.lastPage != m.page {
				return m.gotoNextPage()
			}
		case "s":
			if !m.loading {
				m.loading = true
				m.sortBy = "STARGAZERS"
				m.clearModel()
				return m, tea.Batch(m.spinner.Tick, fetchForks(m.owner, m.name, m.headRef, 0, m.sortBy, nil))
			}
		case "u":
			if !m.loading {
				m.loading = true
				m.sortBy = "UPDATED_AT"
				m.clearModel()
				return m, tea.Batch(m.spinner.Tick, fetchForks(m.owner, m.name, m.headRef, 0, m.sortBy, nil))
			}
		case "enter":
			f := m.forks[m.table.Cursor()]
			u := f.Url

			devNull, err := os.Open(os.DevNull)
			if err != nil {
				log.Fatal(err)
			}
			defer devNull.Close()

			err = openBrowser(u, devNull, devNull)

			// 使いたかったが、ブラウザによる出力が標準出力されてしまい、表示が崩れる
			//b := browser.New("", devNull, devNull)
			//err = b.Browse(u)
			if err != nil {
				fmt.Printf("Failed to open browser: %v\n", err)
			}
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case forksLoadedMsg:
		m.loading = false
		m.forks = msg.forks
		m.totalCount = msg.totalCount
		m.pages[msg.page] = msg.forks
		m.page = msg.page
		m.endCursor = msg.endCursor
		if !msg.hasNext {
			m.lastPage = msg.page
		}
		m.updateTable()
		return m, nil

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *model) gotoNextPage() (tea.Model, tea.Cmd) {
	if _, exists := m.pages[m.page+1]; exists {
		m.page++
		m.loadCachedPage()
		m.table.GotoTop()
		return m, nil
	}

	m.loading = true
	return m, tea.Batch(m.spinner.Tick, fetchForks(m.owner, m.name, m.headRef, m.page+1, m.sortBy, m.endCursor))
}

func (m *model) gotoPreviousPage() (tea.Model, tea.Cmd) {
	m.page--
	m.loadCachedPage()
	return m, nil
}

func (m *model) clearModel() {
	m.forks = nil
	m.pages = make(map[int][]Fork)
	m.lastPage = -1
}

func openBrowser(url string, stdout, stderr io.Writer) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("powershell", "Start-Process", url)
	case "darwin":
		// macOS
		cmd = exec.Command("open", url)
	case "linux":
		providers := []string{"xdg-open", "wslview", "x-www-browser"}
		for _, provider := range providers {
			if _, err := exec.LookPath(provider); err == nil {
				cmd = exec.Command(provider, url)
				break
			}
		}
		if cmd == nil {
			return errors.New("no supported browser launcher found")
		}
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}

func (m *model) View() string {
	if m.loading {
		return fmt.Sprintf("\n Loading... %s\n", m.spinner.View())
	}

	helpMessage := " [↑/k] Up  [↓/j] Down  [←/h] Prev Page  [→/l] Next Page  [s] Sort by Stars  [u] Sort by Updated  [Enter] Open Repo  [q] Quit"

	return fmt.Sprintf(
		"\n GitHub Forks (Page %d, Sort: %s)  Total Forks: %d\n%s\n%s", m.page+1, m.sortBy, m.totalCount, helpMessage, m.table.View(),
	)
}

type forksLoadedMsg struct {
	forks      []Fork
	page       int
	totalCount int
	endCursor  *string
	hasNext    bool
}

func fetchForks(owner, name, headRef string, page int, sortBy string, endCursor *string) tea.Cmd {
	return func() tea.Msg {
		opts := api.ClientOptions{EnableCache: true}
		client, err := api.NewGraphQLClient(opts)
		if err != nil {
			log.Fatal(err)
		}

		var query struct {
			Repository struct {
				Forks struct {
					Nodes []struct {
						NameWithOwner    string
						StargazerCount   int
						ForkCount        int
						UpdatedAt        time.Time
						Url              string
						DefaultBranchRef struct {
							Name    string
							Compare struct {
								AheadBy  int
								BehindBy int
							} `graphql:"compare(headRef: $headRef)"`
						}
					}
					PageInfo struct {
						HasNextPage bool
						EndCursor   *string
					}
					TotalCount int
				} `graphql:"forks(first: 10, after: $endCursor, orderBy: {field: $field, direction: DESC})"`
			} `graphql:"repository(owner: $owner, name: $name)"`
		}

		var endCursorVar *graphql.String
		if endCursor != nil {
			temp := graphql.String(*endCursor)
			endCursorVar = &temp
		} else {
			endCursorVar = (*graphql.String)(nil)
		}

		variables := map[string]interface{}{
			"owner":     graphql.String(owner),
			"name":      graphql.String(name),
			"field":     toRepositoryOrderField(sortBy),
			"headRef":   graphql.String(headRef),
			"endCursor": endCursorVar, // nil if first page
		}

		if err := client.Query("Forks", &query, variables); err != nil {
			log.Fatal(err)
		}

		forks := []Fork{}
		for _, node := range query.Repository.Forks.Nodes {
			forks = append(forks, Fork{
				NameWithOwner:  node.NameWithOwner,
				StargazerCount: node.StargazerCount,
				ForkCount:      node.ForkCount,
				UpdatedAt:      node.UpdatedAt,
				Url:            node.Url,
				BehindBy:       node.DefaultBranchRef.Compare.AheadBy,
				AheadBy:        node.DefaultBranchRef.Compare.BehindBy,
			})
		}

		hasNext := query.Repository.Forks.PageInfo.HasNextPage
		endCursor = query.Repository.Forks.PageInfo.EndCursor

		return forksLoadedMsg{forks: forks, page: page, totalCount: query.Repository.Forks.TotalCount, endCursor: endCursor, hasNext: hasNext}
	}
}

var sortMap = map[string]graphql.RepositoryOrderField{
	"CREATED_AT": graphql.RepositoryOrderFieldCreatedAt,
	"UPDATED_AT": graphql.RepositoryOrderFieldUpdatedAt,
	"PUSHED_AT":  graphql.RepositoryOrderFieldPushedAt,
	"NAME":       graphql.RepositoryOrderFieldName,
	"STARGAZERS": graphql.RepositoryOrderFieldStargazers,
}

func toRepositoryOrderField(sortBy string) graphql.RepositoryOrderField {
	if val, exists := sortMap[sortBy]; exists {
		return val
	}

	return graphql.RepositoryOrderFieldUpdatedAt
}

func (m *model) updateTable() {
	columns := []table.Column{
		{Title: "Repo", Width: 30},
		{Title: "Stars", Width: 10},
		{Title: "Ahead", Width: 10},
		{Title: "Behind", Width: 10},
		{Title: "Updated", Width: 15},
		{Title: "Forks", Width: 10},
	}

	var rows []table.Row
	for _, f := range m.forks {
		rows = append(rows, table.Row{
			f.NameWithOwner,
			strconv.Itoa(f.StargazerCount),
			strconv.Itoa(f.AheadBy),
			strconv.Itoa(f.BehindBy),
			f.UpdatedAt.In(time.Local).Format("2006-01-02"),
			strconv.Itoa(f.ForkCount),
		})
	}

	m.table = table.New(table.WithColumns(columns), table.WithRows(rows), table.WithFocused(true), table.WithHeight(len(rows)+1))
}

func (m *model) loadCachedPage() {
	if forks, exists := m.pages[m.page]; exists {
		m.forks = forks
		m.updateTable()
	}
}

var _ tea.Model = (*model)(nil)

func main() {
	pflag.Parse()

	var owner string
	var name string
	if len(pflag.Args()) > 0 {
		r, err := repository.Parse(pflag.Args()[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		owner = r.Owner
		name = r.Name
	} else {

		r, err := repository.Current()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		owner = r.Owner
		name = r.Name
	}

	defaultBranch, err := getDefaultBranch(owner, name)
	if err != nil {
		if errors.Is(err, ErrNoForks) {
			fmt.Println("No forks found for this repository. Exiting.")
			os.Exit(0)
		}

		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	headRef := fmt.Sprintf("%s:%s", owner, defaultBranch)

	sp := spinner.New()
	initialTable := table.New(
		table.WithColumns([]table.Column{
			{Title: "Repo", Width: 30},
			{Title: "Stars", Width: 10},
			{Title: "Ahead", Width: 10},
			{Title: "Behind", Width: 10},
			{Title: "Updated", Width: 15},
			{Title: "Forks", Width: 10},
		}),
		table.WithFocused(true),
	)

	prog := tea.NewProgram(&model{
		owner:     owner,
		name:      name,
		headRef:   headRef,
		spinner:   sp,
		loading:   true,
		sortBy:    "UPDATED_AT",
		page:      0,
		pages:     make(map[int][]Fork),
		table:     initialTable,
		endCursor: nil,
		lastPage:  -1,
	})

	if _, err := prog.Run(); err != nil {
		log.Fatal(err)
	}
}

var ErrNoForks = errors.New("no forks found")

func getDefaultBranch(owner, name string) (string, error) {
	opts := api.ClientOptions{EnableCache: true}
	client, err := api.NewGraphQLClient(opts)
	if err != nil {
		return "", err
	}

	var query struct {
		Repository struct {
			ForkCount        int
			DefaultBranchRef struct {
				Name string
			}
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner": graphql.String(owner),
		"name":  graphql.String(name),
	}

	err = client.Query("DefaultBranch", &query, variables)
	if err != nil {
		return "", err
	}

	if query.Repository.ForkCount == 0 {
		return "", ErrNoForks
	}

	return query.Repository.DefaultBranchRef.Name, nil
}
