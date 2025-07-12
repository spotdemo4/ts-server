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
      "x86_64-darwin"
      "aarch64-darwin"
    ];
    forSystem = f:
      nixpkgs.lib.genAttrs build-systems (
        system:
          f {
            inherit system;
            pkgs = import nixpkgs {
              inherit system;
              overlays = [
                nur.overlays.default
                nur.legacyPackages."${system}".repos.trev.overlays.renovate
              ];
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
        version = "0.0.17";
        src = ./.;
        goSum = ./go.sum;
        vendorHash = "sha256-FsOmI7WoqiGYzys4XLmEfNPgEeFLrqhe5WtyZf4uzOs=";
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
  in {
    devShells = forSystem ({pkgs, ...}: {
      default = pkgs.mkShell {
        packages = with pkgs; [
          git

          # Nix
          nix-update
          alejandra

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
        ];
      };

      ci = pkgs.mkShell {
        packages = with pkgs; [
          git
          renovate
          podman

          # Nix
          nix-update

          # Go
          go

          # Protobuf
          buf
          protoc-gen-go
          protoc-gen-connect-go
        ];
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
            alejandra
            revive
            sqlfluff
          ];
          checkPhase = ''
            alejandra -c .
            sqlfluff lint
            revive -config revive.toml -set_exit_status ./...
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
        test = ts-server."${system}".overrideAttrs {
          pname = "test";
          doCheck = true;
          dontBuild = true;
          installPhase = ''
            touch $out
          '';
        };
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

          linux_amd64 = goModuleToPlatform default "linux" "amd64";
          linux_arm64 = goModuleToPlatform default "linux" "arm64";
          linux_arm = goModuleToPlatform default "linux" "arm";
          darwin_arm64 = goModuleToPlatform default "darwin" "arm64";
          windows_amd64 = goModuleToPlatform default "windows" "amd64";

          linux_amd64_image = goModuleToImage linux_amd64;
          linux_arm64_image = goModuleToImage linux_arm64;
          linux_arm_image = goModuleToImage linux_arm;
        }
    );
  };
}
