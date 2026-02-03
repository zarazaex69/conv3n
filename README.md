<div align="center">
  <img src="assets/logo/fully_banner.png" alt="Conv3n" width="400"/>
</div>

<div align="center">

![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![TypeScript](https://img.shields.io/badge/-TypeScript-0D1117?style=flat-square&logo=typescript&logoColor=377CC8)
![Bun](https://img.shields.io/badge/-Bun-0D1117?style=flat-square&logo=Bun&logoColor=F3E6D8)
![SQLite](https://img.shields.io/badge/-SQLite-0D1117?style=flat-square&logo=sqlite&logoColor=003B57)

</div>

## About

Conv3n is a lightweight workflow automation engine that lets you build and execute complex workflows using JSON configurations. Chain HTTP requests, transform data, add delays, and create conditional logic with a simple node-based approach.

## Features

- **JSON-based workflows** - Define complex automation flows declaratively
- **Built-in blocks** - HTTP requests, data transforms, delays, conditions, loops
- **Custom blocks** - Extend functionality with TypeScript/Bun runtime
- **SQLite storage** - Lightweight, embedded database
- **Cron triggers** - Schedule workflows with cron expressions

## Fast Start

```bash
# build the engine
make build

# run a workflow
./bin/conv3n run examples/delay_simple.json
```

## Tech Stack

- **Backend**: Go 1.24+ with SQLite
- **Runtime**: Bun for custom blocks
- **SDK**: TypeScript SDK for block development

## Examples

Check the `examples/` directory for workflow samples including HTTP chains, data transforms, and conditional logic.

<div align="center">

---

### Contact

Telegram: [zarazaex](https://t.me/zarazaexe)
<br>
Email: [zarazaex@tuta.io](mailto:zarazaex@tuta.io)
<br>
Site: [zarazaex.xyz](https://zarazaex.xyz)

</div>
