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
    systems.url = "systems";
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    utils = {
      url = "github:numtide/flake-utils";
      inputs.systems.follows = "systems";
    };
    nur = {
      url = "github:nix-community/NUR";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    semgrep-rules = {
      url = "github:semgrep/semgrep-rules";
      flake = false;
    };
    ts-web = {
      url = "git+https://github.com/spotdemo4/ts-web?ref=latest&rev=87d8340bf3504ae331febbabedf2c548efadbb12";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = {
    nixpkgs,
    utils,
    nur,
    semgrep-rules,
    ts-web,
    ...
  }:
    utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {
        inherit system;
        overlays = [nur.overlays.default];
      };
    in rec {
      devShells.default = pkgs.mkShell {
        packages = with pkgs; [
          git
          pkgs.nur.repos.trev.bumper

          # Go
          go
          gotools
          gopls
          golangci-lint
          govulncheck

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
          alejandra
          flake-checker

          # Actions
          action-validator
          prettier
          skopeo
          pkgs.nur.repos.trev.renovate
        ];
        shellHook = pkgs.nur.repos.trev.shellhook.ref;
      };

      checks =
        pkgs.nur.repos.trev.lib.mkChecks {
          lint = {
            src = ./.;
            deps = with pkgs; [
              go
              golangci-lint
              sqlfluff
              alejandra
              prettier
              action-validator
              prettier
              pkgs.nur.repos.trev.renovate
            ];
            script = ''
              golangci-lint run ./...
              sqlfluff lint
              alejandra -c .
              prettier --check .
              action-validator .github/workflows/*
              action-validator .gitea/workflows/*
              renovate-config-validator
              renovate-config-validator .github/renovate-global.json
              renovate-config-validator .gitea/renovate-global.json
            '';
          };

          scan = {
            src = ./.;
            deps = [
              pkgs.nur.repos.trev.opengrep
            ];
            script = ''
              opengrep scan --quiet --error --config="${semgrep-rules}/go"
            '';
          };

          db = {
            src = ./.;
            deps = with pkgs; [
              sqlite
              dbmate
            ];
            script = ''
              export DATABASE_URL=sqlite:$TMP/check.db
              dbmate up
            '';
          };
        }
        // {
          build = packages.default.overrideAttrs {
            doCheck = true;
          };
          shell = devShells.default;
        };

      packages = with pkgs.nur.repos.trev.lib; rec {
        default = pkgs.buildGoModule (finalAttrs: {
          pname = "ts-server";
          version = "0.0.20";
          src = ./.;
          goSum = ./go.sum;
          vendorHash = null;
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
        });

        image = pkgs.dockerTools.streamLayeredImage {
          name = "${default.pname}";
          tag = "${default.version}";
          created = "now";
          contents = [default];
          config = {
            Cmd = [
              "${pkgs.lib.meta.getExe default}"
            ];
          };
        };

        linux-amd64 = go.moduleToPlatform default "linux" "amd64";
        linux-arm64 = go.moduleToPlatform default "linux" "arm64";
        linux-arm = go.moduleToPlatform default "linux" "arm";
        darwin-arm64 = go.moduleToPlatform default "darwin" "arm64";
        windows-amd64 = go.moduleToPlatform default "windows" "amd64";
      };

      formatter = pkgs.alejandra;
    });
}
