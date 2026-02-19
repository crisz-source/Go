package main

import "fmt"

func main() {
    pods := map[string]string{
        "nginx": "Running",
        "api":   "Error",
    }

    status := pods["banco"]
    
    if status == "" {
        fmt.Println("Pod nao encontrado")
    } else {
        fmt.Println("Status:", status)
    }
}
