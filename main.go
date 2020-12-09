package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"

	"github.com/SteveHNH/repbot-go/config"
	"github.com/bwmarrin/discordgo"
	"github.com/jedib0t/go-pretty/table"
	_ "github.com/mattn/go-sqlite3"
)

const dbDriver string = "sqlite3"

const setRep = `UPDATE reputation SET rep = rep + 1, user = ? WHERE username = ?`
const getRep = `SELECT rep FROM reputation WHERE username = ?`
const createUser = `INSERT OR IGNORE into reputation (username, user, rep) values (?, ?, ?)`
const getRank = `SELECT user, rep from reputation ORDER BY rep DESC, user ASC LIMIT 10`
const getTbl = `SELECT name FROM sqlite_master WHERE type='table' AND name='reputation';`
const createTbl = `CREATE TABLE reputation (username TEXT PRIMARY KEY, rep INTEGER DEFAULT 0, user VARCHAR);`
const getUsers = `SELECT username, user FROM 'reputation';`
const updateUserName = `UPDATE reputation SET user = ? WHERE username = ?;`

var configFile string

type repbotClient struct {
	cfg        *config.Config
	ds         *discordgo.Session
	db         *sql.DB
	token      string
	datasource string
}

var client *repbotClient

// init zero parses command line flags
func init() {
	// No default value
	flag.StringVar(&configFile, "c", "", "Specify a path to the config file")
	flag.Parse()
}

func (c *repbotClient) init() (*repbotClient, error) {
	var err error

	cfg := config.Get(configFile)

	c = &repbotClient{
		cfg:        cfg,
		token:      cfg.Token,
		datasource: cfg.DB,
	}

	c.ds, err = discordgo.New("Bot " + c.token)
	if err != nil {
		return c, fmt.Errorf("error creating Discord session: %s", err)
	}

	return c, nil
}

// init the second checks to see if the database needs to be setup before the main function runs
func checkDatabase() {
	tableExists, err := checkTable(client.db)
	if err != nil {
		log.Fatalf("failed checking table 'reputaton' exists: %s", err)
	}

	if !tableExists {
		log.Printf("no table 'reputation' found: creating")
		err := createTable(client.db)
		if err != nil {
			log.Fatalf("failed creating table 'reputation': %s", err)
		}
	}

	tableExists, err = checkTable(client.db)
	if err != nil {
		log.Fatalf("failed checking table 'reputaton' exists: %s", err)
	}

	if !tableExists {
		log.Fatalf("database setup failed: table creation failed")
	}
}

func updateUsers() {
	rows, _ := client.db.Query(getUsers)

	var userName string
	var user string
	var people map[string]string

	people = make(map[string]string)

	for rows.Next() {
		rows.Scan(&userName, &user)
		u, err := client.ds.User(userName)
		if err != nil {
			log.Printf("Error retrieving user details: %s\n", err)
		}

		if u.Username != user {
			log.Printf("Username Mismatch - User: %s, Discord: %s, Repbot: %s. Queuing for update.\n", userName, u.Username, user)

			// Careful - u.Username is the nick according to Discord
			// and userName is the ID according to Repbot's db
			people[userName] = u.Username
		}
	}

	if len(people) == 0 {
		log.Println("No out-of-date users found")
		return
	}

	log.Println("Updating out-of-date users")
	for id, username := range people {
		statement, err := client.db.Prepare(updateUserName)

		if err != nil {
			log.Printf("Unable to prepare sql statement for initRep: %s", err)
		}

		// Careful - u.Username is the nick according to Discord
		// and userName is the ID according to Repbot's db
		// "Update reputation set user = username where id = id"
		_, err = statement.Exec(username, id)
		if err != nil {
			log.Printf("user update execution failed: %s", err)
		}

	}
}

func main() {
	var err error

	client = &repbotClient{}
	client, err = client.init()
	if err != nil {
		log.Fatalln(err)
	}

	client.db, err = sql.Open(dbDriver, client.datasource)
	defer client.db.Close()

	if err != nil {
		log.Fatalln(err)
	}

	// Check if the databse is setup
	checkDatabase()

	client.ds.AddHandler(messageCreate)
	client.ds.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)

	err = client.ds.Open()
	defer client.ds.Close()
	if err != nil {
		log.Fatalf("error opening Discord connection: %s", err)
	}

	log.Println("checking for user updates from Discord")
	updateUsers()
	log.Println("completed checking users for updates")

	log.Println("Bot is now running. Press CTRL+C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore repbot's own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	log.Printf("received message: \"%s\" - @%s - %s", m.Content, m.Author.Username, m.Author.ID)

	var matched bool
	repAdd := `^\!rep\s\<\@\!*\d+\>\s*`
	repRank := `^\!rep\srank\s*`
	repPing := `^\!rep\sping\s*`

	matched, _ = regexp.MatchString(repPing, m.Content)
	if matched {
		log.Printf("Ping request received")
		s.ChannelMessageSend(m.ChannelID, "pong")
		return
	}

	matched, _ = regexp.MatchString(repRank, m.Content)
	if matched {
		log.Printf("Rank request received")
		repRankAll(m, s)
		return
	}

	matched, _ = regexp.MatchString(repAdd, m.Content)
	if matched {
		log.Printf("Rep increase request received")
		if m.Author.ID != m.Mentions[0].ID {
			repInc(m.Mentions[0], m, s)
			return
		}

		log.Printf("Ignoring greedy rep request")
		s.ChannelMessageSend(m.ChannelID, "You can't update your own rep")
	}
}

// checkUser returns true if the user exists in the db, or false if not
func checkUser(u *discordgo.User, db *sql.DB) (result bool, err error) {
	rows, err := client.db.Query(getRep, u.ID)
	if err != nil {
		return false, err
	}

	var repValue int
	for rows.Next() {
		rows.Scan(&repValue)
	}
	if repValue > 0 {
		return true, nil
	}
	return false, nil
}

// repRankAll queries the database for all users, orders them by rep and sends a table with this info to the channel
func repRankAll(m *discordgo.MessageCreate, s *discordgo.Session) {

	t := table.NewWriter()
	t.AppendHeader(table.Row{"Rep", "User"})

	rows, _ := client.db.Query(getRank)

	var userName string
	var repValue int

	for rows.Next() {
		rows.Scan(&userName, &repValue)
		t.AppendRow([]interface{}{strconv.Itoa(repValue), userName})
	}

	t.SetStyle(table.StyleLight)
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```\n%s\n```", t.Render()))

	return

}

func repInc(u *discordgo.User, m *discordgo.MessageCreate, s *discordgo.Session) {

	// Check if user exists
	usercheck, err := checkUser(u, client.db)
	if err != nil {
		log.Printf("Something broke with userCheck: %s", err)
		return
	}

	// User doesn't exist yet
	if usercheck == false {
		statement, err := client.db.Prepare(createUser)
		if err != nil {
			log.Printf("Unable to prepare sql statement for initRep: %s", err)
			s.ChannelMessageSend(m.ChannelID, "Could not initialize new user "+u.Username)
			return
		}

		_, err = statement.Exec(u.ID, u.Username, 0)
		if err != nil {
			log.Printf("initRep execution failed: %s", err)
			s.ChannelMessageSend(m.ChannelID, "Could not initialize user "+u.Username)
			return
		}
		log.Printf("created new user: %s", u.Username)
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Welcome new user %s!", u.Username))
	}

	// User already exists
	statement, err := client.db.Prepare(setRep)
	if err != nil {
		log.Printf("Unable to prepare sql statement for repInc: %s", err)
		s.ChannelMessageSend(m.ChannelID, "Could not update rep for "+u.Username)
		return
	}

	_, err = statement.Exec(u.Username, u.ID)
	if err != nil {
		log.Printf("repInc execution failed: %s", err)
		s.ChannelMessageSend(m.ChannelID, "Could not update rep for "+u.Username)
		return
	}

	rows, _ := client.db.Query(getRep, u.ID)
	var repValue int
	for rows.Next() {
		rows.Scan(&repValue)
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Rep increased to %s for %s", strconv.Itoa(repValue), u.Username))
	return
}

// checkTable checks to see if a table named "reputation" exists
func checkTable(db *sql.DB) (bool, error) {
	var table string = "reputation"
	var t string

	rows, _ := db.Query(getTbl)
	for rows.Next() {
		rows.Scan(&t)
	}

	if t == table {
		return true, nil
	}

	return false, nil
}

// createTable creates the "reputation" table with the schema described in the initDB constant
func createTable(d *sql.DB) error {
	_, err := d.Exec(createTbl)
	if err != nil {
		return err
	}

	return nil
}
