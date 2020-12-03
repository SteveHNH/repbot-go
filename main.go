package main

import (
	"database/sql"
	"errors"
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

const setRep = `UPDATE reputation SET rep = rep + 1, user = ? WHERE username = ?`
const getRep = `SELECT rep FROM reputation WHERE username = ?`
const initRep = `INSERT OR IGNORE into reputation (username, user, rep) values (?, ?, ?)`
const getRank = `SELECT user, rep from reputation ORDER BY rep DESC, user ASC`
const checkDb = `SELECT name FROM sqlite_master WHERE type='table' AND name='reputation';`
const initDb = `CREATE TABLE reputation (username TEXT PRIMARY KEY, rep INTEGER DEFAULT 0, user VARCHAR);`

// init checks to see if the database needs to be setup before the main function runs
func init() {
	// Check the sqlite db is setup correctly
	err := dbCheck()
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	cfg := config.Get()

	dg, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		log.Fatalf("error creating Discord session: %s", err)
	}

	dg.AddHandler(messageCreate)

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)

	err = dg.Open()
	defer dg.Close()

	if err != nil {
		log.Fatalf("error opening Discord connection: %s", err)
	}

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

func checkUser(u *discordgo.User, db *sql.DB) (result bool, err error) {
	rows, err := db.Query(getRep, u.ID)
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

func repRankAll(m *discordgo.MessageCreate, s *discordgo.Session) {
	db, _ := sql.Open("sqlite3", config.Get().DB)
	defer db.Close()

	t := table.NewWriter()
	t.AppendHeader(table.Row{"Rep", "User"})

	rows, _ := db.Query(getRank)

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
	db, _ := sql.Open("sqlite3", config.Get().DB)
	defer db.Close()

	// Check if user exists
	usercheck, err := checkUser(u, db)
	if err != nil {
		log.Printf("Something broke with userCheck: %s", err)
		return
	}

	// User doesn't exist yet
	if usercheck == false {
		statement, err := db.Prepare(initRep)
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
	statement, err := db.Prepare(setRep)
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

	rows, _ := db.Query(getRep, u.ID)
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

	rows, _ := db.Query(checkDb)
	for rows.Next() {
		rows.Scan(&t)
	}

	if t == table {
		return true, nil
	}

	return false, nil
}

// dbInit creates the "reputation" table with the schema described in the initDB constant
func dbInit(db *sql.DB) error {
	_, err := db.Exec(initDb)
	if err != nil {
		return err
	}

	return nil
}

// dbCheck opens a db connection, checks to see if the right table exists, and creates one if not
func dbCheck() error {
	db, err := sql.Open("sqlite3", config.Get().DB)
	defer db.Close()
	if err != nil {
		return err
	}

	tableExists, err := checkTable(db)
	if err != nil {
		return err
	}

	if !tableExists {
		log.Printf("No table 'reputation' found, creating...")
		err := dbInit(db)
		if err != nil {
			return err
		}
	}

	tableExists, err = checkTable(db)
	if err != nil {
		return err
	}
	if !tableExists {
		return errors.New("Database setup failed: table creation failed")
	}

	return nil
}
