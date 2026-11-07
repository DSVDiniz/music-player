# Inhuman Resources
Jogo estilo Papers Please onde você é uma muié de RH tentando ao máximo não contratar ninguém.
## gameplay loop
1. a vaga de emprego é descrita para o jogador, com todos os parâmetros do que deveria ser checado, idade, sexo, experiência, etc
2. um certo número de currículos são dados para o jogador investigar
3. qualquer parâmetro ou desculpa que possa ser usado para negar um candidato pode ser utilizado, mas se o jogador escolher uma desculpa errada ou negar um currículo sem problemas, ele perde pontos
4. depois dos currículos vem a entrevista inicial, aqui o jogador pode escolher perguntas maliciosas para fazer os candidatos errarem ou demonstrarem fraquezas ou perceber comportamentos para após a entrevista dar um desculpa para não contratar o candidato, por exemplo:
    - se o candidato olhar para o decote da personagem principal, o jogador pode dizer no fim que o candidato é machista, e não combina com a cultura da empresa
    - se ele errar perguntas levemente técnicas também seria razão para a desqualificação
    - se o candidato parece estar colando ou trapaceando
    - se o candidato aparece atrasado para a entrevista
    - etc
5. depois da entrevista inicial, vem a entrevista técnica, onde o jogador assiste uma entrevista entre o técnico e o candidato, novameente tentando achar um desculpa para não contratar
6. poderia existir outras etapas extras ou intermediárias para dar mais chances para o jogador não contratar o candidato, como uma busca dos posts do candidato online para ver se ele tem opiniões politicas, fala mal da empresa, usa IA para fazer posts no linkedin, etc. Nesses casos seria um time attack.
7. se no fim da rodada de contratação o jogador perder muitos pontos, ele é demitido

## aspectos roguelike
Depois de cada rodada o jogador poderia escolher skills que facilitam a vida dele, filtros de IA nos curriculos, informações detalhadas para saber quais comportamentos dos candidatos podem ser considerados "ruins para a companhia" (só mais desculpas para não contratar)

## historia
Como no Papers Please, poderia ter uma história maior do fundo, com consequencias para o jogador caso ele contrate pessoas ou não.

## desenvolvimento/problemas
- não precisa ser um jogo muito bonito inicialmente para testes e protótipos, mas por ser um jogo simples, precisa ter um estilo marcante
- precisa ser engraçado, mas nesse caso não vejo tantos problemas já que é muito fácil achar conteúdo sobre isso na internet e há milhões de memes sobre isso
- não precisa de um engine pesado, sinceramente daria pra fazer do zero, mas acho que godot seria o mais simples e rápido após o protótipo


# Protótipo
- tela inicial
- tela do curriculo
- tela da entrevista
- tela mídia social
- conversas
- game over