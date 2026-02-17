{ lib
, buildGoModule
, fetchFromGitHub
}:

buildGoModule rec {
  pname = "pepebot";
  version = "0.4.0";

  src = fetchFromGitHub {
    owner = "pepebot-space";
    repo = "pepebot";
    rev = "v${version}";
    hash = ""; # Will be filled by nix-prefetch-url
  };

  vendorHash = ""; # Will be filled automatically

  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
    "-X main.buildTime=1970-01-01T00:00:00Z"
  ];

  # Disable CGO for pure static binary
  CGO_ENABLED = 0;

  # Build only the main binary
  subPackages = [ "cmd/pepebot" ];

  # Tests require network access and external services
  doCheck = false;

  meta = with lib; {
    description = "Ultra-lightweight personal AI agent";
    longDescription = ''
      Pepebot is an ultra-lightweight and efficient personal AI agent.
      It supports multiple LLM providers (Anthropic, OpenAI, OpenRouter, Groq, Gemini, etc.),
      multiple chat channels (Telegram, Discord, WhatsApp, Feishu, MaixCam),
      and an extensible tool/skill system with Android automation via ADB.
    '';
    homepage = "https://github.com/pepebot-space/pepebot";
    changelog = "https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md";
    license = licenses.mit;
    maintainers = with maintainers; [ ];
    platforms = platforms.unix ++ platforms.darwin;
    mainProgram = "pepebot";
  };

  postInstall = ''
    # Install shell completions if available
    if [ -f completion.bash ]; then
      installShellCompletion --bash completion.bash
    fi
    if [ -f completion.zsh ]; then
      installShellCompletion --zsh completion.zsh
    fi
    if [ -f completion.fish ]; then
      installShellCompletion --fish completion.fish
    fi
  '';
}
