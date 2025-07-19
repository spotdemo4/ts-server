{
  description = "Trevstack Server";

  nixConfig = {
    extra-substituters = [
      "https://trevnur.cachix.org"
    ];
    extra-trusted-public-keys = [
      "trevnur.cachix.org-1:hBd15IdszwT52aOxdKs5vNTbq36emvEeGqpb25Bkq6o="
    ];
  };

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    nur = {
      url = "github:nix-community/NUR";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    semgrep-rules = {
      url = "github:semgrep/semgrep-rules";
      flake = false;
    };
    ts-web = {
      url = "github:spotdemo4/ts-web/latest";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = {
    nixpkgs,
    nur,
    semgrep-rules,
    ts-web,
    ...
  }: let
    build-systems = [
      "x86_64-linux"
      "aarch64-linux"
      "aarch64-darwin"
    ];
    forSystem = f:
      nixpkgs.lib.genAttrs build-systems (
        system:
          f {
            inherit system;
            pkgs = import nixpkgs {
              inherit system;
              overlays = [nur.overlays.default];
            };
          }
      );

    ts-server = forSystem ({
      pkgs,
      system,
      ...
    }:
      pkgs.buildGoModule (finalAttrs: {
        pname = "ts-server";
        version = "0.0.20";
        src = ./.;
        goSum = ./go.sum;
        vendorHash = "sha256-xryKCimGbunISAy8qnBzD3nZuHav/eUBAxPgkXYTydM=";
        env.CGO_ENABLED = 0;

        preBuild = ''
          cp -r ${ts-web.packages."${system}".default} client
        '';

        meta = {
          description = "A simple GO CRUD app";
          mainProgram = "ts-server";
          homepage = "https://github.com/spotdemo4/ts-server";
          changelog = "https://github.com/spotdemo4/ts-server/releases/tag/v${finalAttrs.version}";
          license = pkgs.lib.licenses.mit;
          platforms = pkgs.lib.platforms.all;
        };
      }));
  in rec {
    devShells = forSystem ({pkgs, ...}: {
      default = pkgs.mkShell {
        packages = with pkgs; [
          git

          # Go
          go
          gotools
          gopls
          revive

          # Database
          sqlite
          dbmate
          sqlfluff
          pkgs.nur.repos.trev.bobgen

          # Protobuf
          buf
          protoc-gen-go
          protoc-gen-connect-go

          # Nix
          nix-update
          alejandra

          # Actions
          renovate
          action-validator
          prettier
          docker-client
        ];
        shellHook = ''
          echo "nix flake check --accept-flake-config" > .git/hooks/pre-commit
          chmod +x .git/hooks/pre-commit
        '';
      };
    });

    checks = forSystem ({
      pkgs,
      system,
      ...
    }:
      pkgs.nur.repos.trev.lib.mkChecks {
        lint = {
          src = ./.;
          nativeBuildInputs = with pkgs; [
            revive
            sqlfluff
            alejandra
            renovate
            action-validator
            prettier
          ];
          checkPhase = ''
            revive -config revive.toml -set_exit_status ./...
            sqlfluff lint
            alejandra -c .
            renovate-config-validator
            action-validator .github/workflows/*
            action-validator .gitea/workflows/*
            action-validator .forgejo/workflows/*
            prettier --check .
          '';
        };

        scan = {
          src = ./.;
          nativeBuildInputs = [
            pkgs.nur.repos.trev.opengrep
          ];
          checkPhase = ''
            mkdir -p "$TMP/scan"
            HOME="$TMP/scan"
            opengrep scan --quiet --error --config="${semgrep-rules}/go"
          '';
        };

        db = {
          src = ./.;
          nativeBuildInputs = with pkgs; [
            sqlite
            dbmate
          ];
          checkPhase = ''
            export DATABASE_URL=sqlite:$TMP/check.db
            dbmate up
          '';
        };
      }
      // {
        build = ts-server."${system}".overrideAttrs {
          doCheck = true;
          installPhase = ''
            touch $out
          '';
        };

        shell = devShells."${system}".default;
      });

    formatter = forSystem ({pkgs, ...}: pkgs.alejandra);

    packages = forSystem (
      {
        pkgs,
        system,
        ...
      }:
        with pkgs.nur.repos.trev.lib; rec {
          default = ts-server."${system}";

          linux-amd64 = goModuleToPlatform default "linux" "amd64";
          linux-arm64 = goModuleToPlatform default "linux" "arm64";
          linux-arm = goModuleToPlatform default "linux" "arm";
          darwin-arm64 = goModuleToPlatform default "darwin" "arm64";
          windows-amd64 = goModuleToPlatform default "windows" "amd64";

          linux-amd64-image = goModuleToImage linux-amd64;
          linux-arm64-image = goModuleToImage linux-arm64;
          linux-arm-image = goModuleToImage linux-arm;
        }
    );
  };
}
