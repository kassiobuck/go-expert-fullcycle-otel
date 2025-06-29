# Desafio
Desenvolver um sistema em Go que receba um CEP, identifica a cidade e retorna o clima atual (temperatura em graus celsius, fahrenheit e kelvin) juntamente com a cidade. Esse sistema deverá implementar OTEL(Open Telemetry) e Zipkin.

## Pré requisitos

- [Go](https://golang.org/doc/install)
- [Docker](https://www.docker.com/get-started)
- [WeatherApi KEY](https://www.weatherapi.com)

## Executando o projeto em modo de desenvolvimento

#### Service A | Input
Acessar a pasta  service-input e executar o comando: 
```shell
    go run main.go
```

#### Service B | Orchestrator
- Abrir o arquivo `env/service-orchestrator.dev.env` e preencher a constante `WEATHER_API_KEY` com uma WeatherApi KEY válida;
- Acessar a pasta service-orchestrator e executar o comando:
```shell
    go run main.go 
```

#### OTEL
Execute o comando: 
```shell
    docker run -d -p 4317:4317 otel/opentelemetry-collector:0.128.0`
```
*Otel está configurado como coletor para integrar com zipkin, Zipkin disponível apenas no modo de produção*

## Executando o projeto em modo de produção
- Abrir o arquivo `env/service-orchestrator.prod.env` e preencher a constante `WEATHER_API_KEY` com uma WeatherApi KEY válida;

- Executar o comando:
```shell
    docker compose up
```

## Testando API
Abra o arquivo `api.http`, nele é possivel acessar a API e executar os testes já realizados.
Para novos testes basta substituir o valor de `"cep"`.

## Zipkin
Abra o endereço: http://localhost:9411/zipkin/
*Disponível apenas em modo de produção.*