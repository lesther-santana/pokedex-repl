package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
)

type cliCommand struct {
	name        string
	description string
	callback    func() func(c *params) error
}

type params struct {
	cache       *Cache
	current     string
	next        string
	prev        string
	arg         string
	userPokedex map[string]pokemon
}

type apiResponse struct {
	Count    int       `json:"count"`
	Next     string    `json:"next"`
	Previous string    `json:"previous"`
	Results  []results `json:"results"`
}

type results struct {
	Name string `json:"name"`
}

type areaResponse struct {
	Name              string             `json:"name"`
	PokemonEncounters []pokemonEncounter `json:"pokemon_encounters"`
}

type pokemonEncounter struct {
	Pokemon pokemon `json:"pokemon"`
}
type pokemon struct {
	Id             int           `json:"id"`
	Name           string        `json:"name"`
	BaseExperience int           `json:"base_experience"`
	Height         int           `json:"height"`
	Weight         int           `json:"weight"`
	Stats          []pokemonStat `json:"stats"`
	Types          []pokemonType `json:"types"`
}

type pokemonType struct {
	Slot int              `json:"slot"`
	Type namedAPIResource `json:"type"`
}

type pokemonStat struct {
	Stat     namedAPIResource `json:"stat"`
	Effort   int              `json:"effort"`
	BaseStat int              `json:"base_stat"`
}

type namedAPIResource struct {
	Name string `json:"name"`
	Url  string `json:"url"`
}

func help() func(*params) error {
	return func(c *params) error {
		fmt.Println()
		fmt.Println("Welcome to the Pokedex v1.0")
		fmt.Println("Usage:")
		fmt.Println()
		commands := createCommands()
		for _, v := range commands {
			fmt.Printf("%s: %s\n", v.name, v.description)
		}
		fmt.Println("")
		return nil
	}
}

func exitPoke() func(*params) error {
	return func(c *params) error {
		fmt.Print("Closing Pokedex ... See you later Ash!")
		os.Exit(0)
		return nil
	}
}

func createCommands() map[string]cliCommand {
	commands := map[string]cliCommand{
		"help": {
			name:        "help",
			description: "Displays a help message",
			callback:    help,
		},
		"exit": {
			name:        "exit",
			description: "Close the Pokedex",
			callback:    exitPoke,
		},
		"map": {
			name:        "map",
			description: "displays the names of 10 location areas in the Pokemon world",
			callback:    apiMap,
		},
		"mapb": {
			name:        "mapb",
			description: "It displays the previous 10 locations.",
			callback:    apiMapBack,
		},
		"explore": {
			name:        "explore",
			description: "see a list of all the Pokémon in a given area",
			callback:    explore,
		},
		"catch": {
			name:        "catch",
			description: "catch some pokemon!",
			callback:    catch,
		},
		"inspect": {
			name:        "inspect",
			description: "Allow players to see details about a Pokemon if they have seen it",
			callback:    inspect,
		},
		"pokedex": {
			name:        "pokedex",
			description: "all the names of the Pokemon the user has",
			callback:    pokedex,
		},
	}
	return commands
}

func showMap(m []results) {
	for _, v := range m {

		fmt.Println(v.Name)
	}
}

func getLocations(url string) (*apiResponse, error) {
	res, err := http.Get(url)
	if err != nil {
		fmt.Println("Error", err)
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	body, err := io.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil || res.StatusCode >= 400 {
		fmt.Println("Error", err)
		return nil, fmt.Errorf("error reading response body: %w", err)
	}
	response := apiResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("Error", err)
		return nil, fmt.Errorf("error unmarshalling response: %w", err)
	}
	return &response, nil
}

func apiMap() func(*params) error {
	return func(p *params) error {
		endpoint := p.next
		if endpoint == "" {
			endpoint = p.current
		}
		response, err := getLocations(endpoint)
		if err != nil {
			return err
		}
		p.prev = response.Previous
		p.current = endpoint
		p.next = response.Next
		showMap(response.Results)
		return nil
	}
}

func apiMapBack() func(*params) error {
	return func(p *params) error {
		endpoint := p.prev
		if endpoint == "" {
			return errors.New("error: first page")
		}
		response, err := getLocations(endpoint)
		if err != nil {
			return err
		}
		p.next = response.Previous
		p.current = endpoint
		p.prev = response.Next
		showMap(response.Results)
		return nil
	}
}

func getArea(p *params) (*[]pokemonEncounter, error) {
	// Check cache first
	if cachedBody, ok := p.cache.Get(p.arg); ok {
		// Cached data exists, use it
		var response areaResponse
		if err := json.Unmarshal(cachedBody, &response); err != nil {
			return nil, fmt.Errorf("error unmarshalling cached response: %w", err)
		}
		return &response.PokemonEncounters, nil
	}

	// If not in cache, fetch from API
	url := fmt.Sprintf("https://pokeapi.co/api/v2/location-area/%s/", p.arg)
	res, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error sending request to %s: %w", url, err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("received error status code: %d from %s", res.StatusCode, url)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body from %s: %w", url, err)
	}

	// Unmarshal and check for errors
	var response areaResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error unmarshalling response from %s: %w", url, err)
	}

	// Cache the new response
	p.cache.Add(p.arg, body)

	return &response.PokemonEncounters, nil
}

func explore() func(*params) error {
	return func(p *params) error {
		response, err := getArea(p)
		if err != nil {
			return err
		}
		if len(*response) == 0 {
			fmt.Println("No pokemons found")
			return nil
		}
		println("Found Pokemon: ")
		for _, v := range *response {
			pretty := fmt.Sprintf(" - %s", v.Pokemon.Name)
			println(pretty)
		}
		return nil
	}
}

func getPokemon(p *params) (*pokemon, error) {
	// Check cache first
	if cachedBody, ok := p.cache.Get(p.arg); ok {
		// Cached data exists, use it
		var response pokemon
		if err := json.Unmarshal(cachedBody, &response); err != nil {
			return nil, fmt.Errorf("error unmarshalling cached response: %w", err)
		}
		return &response, nil
	}

	// If not in cache, fetch from API
	url := fmt.Sprintf("https://pokeapi.co/api/v2/pokemon/%s/", p.arg)
	res, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error sending request to %s: %w", url, err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("received error status code: %d from %s", res.StatusCode, url)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body from %s: %w", url, err)
	}

	// Unmarshal and check for errors
	var response pokemon
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error unmarshalling response from %s: %w", url, err)
	}

	// Cache the new response
	p.cache.Add(p.arg, body)

	return &response, nil
}

func catchProbability(baseExperience, maxBaseExperience int) float64 {
	return 1.0 - (float64(baseExperience) / float64(maxBaseExperience))
}

func catch() func(*params) error {
	return func(p *params) error {
		poke, err := getPokemon(p)
		if p.userPokedex == nil {
			p.userPokedex = make(map[string]pokemon)
		}
		if err != nil {
			return err
		}
		pretty := fmt.Sprintf("Throwing a Pokeball at %s (%d)...", poke.Name, poke.BaseExperience)
		println(pretty)
		proba := catchProbability(poke.BaseExperience, 500)
		result := fmt.Sprintf("%s has escaped!", poke.Name)
		if rand.Float64() < proba {
			p.userPokedex[poke.Name] = *poke
			result = fmt.Sprintf("%s was caught!", poke.Name)
		}
		println(result)
		return nil
	}
}

func inspect() func(*params) error {
	return func(p *params) error {
		pokemon, ok := p.userPokedex[p.arg]
		if !ok {
			pretty := fmt.Sprintf("%s not caught yet!", p.arg)
			fmt.Println(pretty)
			return nil
		}
		fmt.Printf("Name: %s\n", pokemon.Name)
		fmt.Printf("Height: %d\n", pokemon.Height)
		fmt.Printf("Weight: %d\n", pokemon.Weight)
		fmt.Println("Stats: ")
		for _, v := range pokemon.Stats {
			fmt.Printf(" - %s: %d\n", v.Stat.Name, v.BaseStat)
		}
		fmt.Println("Types: ")
		for _, ptype := range pokemon.Types {
			fmt.Printf(" - %s\n", ptype.Type.Name)
		}
		return nil
	}
}

func pokedex() func(*params) error {
	return func(p *params) error {
		if p.userPokedex == nil {
			fmt.Println("Your Pokedex is empty")
			return nil
		}
		println("Your Pokedex")
		for k := range p.userPokedex {
			fmt.Printf(" - %s\n", k)
		}
		return nil
	}
}

func main() {
	fmt.Println("Pokedex v1.0 running ......")
	pagination := params{}
	pagination.current = "https://pokeapi.co/api/v2/location-area?limit=10"
	pagination.cache = NewCache(5 * time.Minute)

	commandMap := createCommands()
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("Pokedex > ")
		scanner.Scan()
		input := scanner.Text()
		words := strings.Fields((input))
		if len(words) > 1 {
			pagination.arg = words[1]
		}
		command, ok := commandMap[words[0]]
		if !ok {
			fmt.Println("Invalid commannd...Try Again")
			continue
		}
		err := command.callback()(&pagination)
		if err != nil {
			fmt.Println(err)
		}
	}
}
