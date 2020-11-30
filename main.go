package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"

	"github.com/SteveHNH/repbot-go/config"
	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
)

const setRep = `UPDATE reputation SET rep = rep + 1, user = ? WHERE username = ?`
const getRep = `SELECT rep FROM reputation WHERE username = ?`
const initRep = `INSERT OR IGNORE into reputation (username, user, rep) values (?, ?, ?)`

func main() {
	cfg := config.Get()
	dg, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		fmt.Println("error creating Discord session:,", err)
		return
	}

	dg.AddHandler(messageCreate)

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	fmt.Println("Bot is now running. Press CTRL+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	//notes for me
	// mention.username: stephen
	// mention ID: 123423452345
	// author ID: 1231242134

	var matched bool
	//trigger := `^\!rep.*`
	repAdd := `^\!rep\s\<\@\!*\d+\>\s*`
	repPing := `^\!rep\sping\s*`

	if m.Author.ID == s.State.User.ID {
		return
	}
	//TODO: Remove after debugging
	fmt.Println(m.Content)
	matched, _ = regexp.MatchString(repPing, m.Content)
	if matched {
		s.ChannelMessageSend(m.ChannelID, "pong")
	}

	matched, _ = regexp.MatchString(repAdd, m.Content)
	if matched {
		if m.Author.ID != m.Mentions[0].ID {
			repInc(m.Mentions[0], m, s)
		}

		s.ChannelMessageSend(m.ChannelID, "You can't update your own rep")
	}
}

func checkUser(u *discordgo.User, db *sql.DB) (result bool, err error) {
	rows, err := db.Query(getRep, u.ID)
	if err != nil {
		fmt.Printf("Couldn't check for user: %s", err)
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

func repInc(u *discordgo.User, m *discordgo.MessageCreate, s *discordgo.Session) {
	db, _ := sql.Open("sqlite3", config.Get().DB)
	defer db.Close()

	// Check if user exists
	usercheck, err := checkUser(u, db)
	if err != nil {
		fmt.Printf("Something broke with userCheck: %s", err)
		return
	}

	// User doesn't exist yet
	if usercheck == false {
		statement, err := db.Prepare(initRep)
		if err != nil {
			fmt.Printf("Unable to prepare sql statement for initRep: %s", err)
			s.ChannelMessageSend(m.ChannelID, "Could not initialize new user "+u.Username)
			return
		}

		_, err = statement.Exec(u.ID, u.Username, 1)
		if err != nil {
			fmt.Printf("initRep execution failed: %s", err)
			s.ChannelMessageSend(m.ChannelID, "Could not initialize user "+u.Username)
			return
		}
		s.ChannelMessageSend(m.ChannelID, "Intialized new user with 1 rep! Congrats, "+u.Username)
		return
	}

	// User already exists
	statement, err := db.Prepare(setRep)
	if err != nil {
		fmt.Printf("Unable to prepare sql statement for repInc: %s", err)
		s.ChannelMessageSend(m.ChannelID, "Could not update rep for "+u.Username)
		return
	}

	_, err = statement.Exec(u.Username, u.ID)
	if err != nil {
		fmt.Printf("repInc execution failed: %s", err)
		s.ChannelMessageSend(m.ChannelID, "Could not update rep for "+u.Username)
		return
	}

	rows, _ := db.Query(getRep, u.ID)
	var repValue int
	for rows.Next() {
		rows.Scan(&repValue)
	}
	s.ChannelMessageSend(m.ChannelID, "Rep increased to "+strconv.Itoa(repValue)+" for "+u.Username)
	return
}
