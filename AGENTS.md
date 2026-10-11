# Agents.md

This is a Golang project named SWORD-GO. Its main goal is to create virtual servers on cloud providers and deploy and
manage WordPress on them.

## Project structure

- Main entripoint: [main.go](main.go).
- Other go sorce files are in [interna](internal) folder.
- Besides the standard [README](README.md), you can find more documentation in the [docs](docs)

## Technological stack

This project creates a single Golang executable. If you start it with `go run . web` it runs a web application to
operate the console. One of the fixed constraints of the project is that the web UI must use just [htmx](https://htmx.org/).
A minimum amount of JavaScript is admitted, but only if is strictly necessary.

The program can also be used as a command line tool for manually creating virtual servers on various cloud providers.

## Code conventions
All the code is written in English, comments included.
