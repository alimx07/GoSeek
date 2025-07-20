package main

// --> In some apps, this process is known as a daemon but for me snake is better
// Snake is a lightweight background process
// That will be ON when GoSeek is off to write any changes
// in folders already indexed to txt files
// then the indexes will be updated when The App goes on again

// Testing Start Script for the Snake
// Not fully functional & still in testing

// TODO :
// Add signal to End Snakes watching
// Handle errors and write to log
// Test in different situations
func main() {
	snake := SnakeConfig{}
	err := snake.LoadConfig("config.yaml")
	if err != nil {
		print(err)
		return
	}
	snake.StartWatchers()
}
