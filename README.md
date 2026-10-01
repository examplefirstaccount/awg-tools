# awg-tools

Два независимых инструмента:

- **`awgproxy`** — поднимает локальный HTTP/SOCKS5-прокси из
  AmneziaWG/WireGuard-конфига. `--bypass` дополнительно прогоняет трафик
  через BPF-релей, чтобы обойти фильтры Cisco AnyConnect на рабочем Mac.
- **`awg-fetch`** — скачивает конфиги с hide.mn. Существует потому, что у
  меня есть подписка на hide.mn и все конфиги я беру оттуда.
  `awgproxy` в нём не нуждается.

**Без AWG/WireGuard-конфига `awgproxy` работать не будет.** Взять можно
откуда угодно — свой провайдер, задать вручную, или через `awg-fetch`, если
пользуешься hide.mn

## Установка

Нужно:
- macOS
- `go` + `libpcap`, только чтобы собрать relay
- [`wireproxy`](https://github.com/pufferffish/wireproxy) — для AWG нужен
  форк [artem-russkikh/wireproxy-awg](https://github.com/artem-russkikh/wireproxy-awg),
  его нет в Homebrew

Установка:
```bash
brew install go libpcap

mkdir -p ~/.local/bin
ARCH=$(uname -m); [ "$ARCH" = "x86_64" ] && ARCH=amd64
curl -L "https://github.com/artem-russkikh/wireproxy-awg/releases/latest/download/wireproxy_darwin_${ARCH}.tar.gz" | tar -xz -C ~/.local/bin wireproxy
chmod +x ~/.local/bin/wireproxy

git clone https://github.com/examplefirstaccount/awg-tools.git
cd awg-tools
./install.sh
```

Установит `awgproxy` и `awg-fetch` в `~/.local/bin`, relay — в
`~/.local/libexec/awgproxy`, настроит sudoers-правило для `--bypass`, и создаст
`~/.config/awgproxy/{configs,backups}`. Сам репозиторий — просто исходники,
после установки его можно удалить. После любых изменений надо перезапускать `install.sh`.

## Как пользоваться

### 1. Конфиги

`awgproxy` читает `.conf`-файлы из `~/.config/awgproxy/configs/`. Файл на
диске хранится с расширением (`myserver.conf`), а команде передаётся имя
**без** `.conf` (`awgproxy myserver`):

```bash
mkdir -p ~/.config/awgproxy/configs
cp мой-конфиг.conf ~/.config/awgproxy/configs/myserver.conf
```

Альтернатива для тех, кто берёт конфиги с hide.mn:

```bash
mkdir -p ~/.config/awgproxy && echo "твой-код-hideme" > ~/.config/awgproxy/code
awg-fetch [host]  # скачать конфиги (host по умолчанию hide.mn)
```

### 2. awgproxy

```bash
awgproxy list        # список доступных конфигов (без расширения .conf), например myserver
awgproxy myserver [--socks5] [--port N] [--bypass]
```

## Удаление

```bash
./uninstall.sh
```

Удаляет установленные бинарники и sudoers-правило. Конфиги не трогает.
