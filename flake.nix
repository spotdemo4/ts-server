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
    ts-web = {
      url = "github:spotdemo4/ts-web/latest";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = {
    nixpkgs,
    nur,
    ts-web,
    ...
  }: let
    pname = "ts-server";
    version = "0.0.17";

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

    host-systems = [
      {
        GOOS = "linux";
        GOARCH = "amd64";
      }
      {
        GOOS = "linux";
        GOARCH = "arm64";
      }
      {
        GOOS = "linux";
        GOARCH = "arm";
      }
      {
        GOOS = "windows";
        GOARCH = "amd64";
      }
      {
        GOOS = "darwin";
        GOARCH = "amd64";
      }
      {
        GOOS = "darwin";
        GOARCH = "arm64";
      }
    ];
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

    checks = forSystem ({pkgs, ...}: {
      lint = with pkgs;
        runCommandLocal "check-lint" {
          nativeBuildInputs = with pkgs; [
            alejandra
            revive
            sqlfluff
          ];
        } ''
          cd ${./.}
          HOME=$PWD

          alejandra -c .
          sqlfluff lint
          revive -config revive.toml -set_exit_status ./...

          touch $out
        '';

      scan = with pkgs; let
        rules = pkgs.fetchFromGitHub {
          owner = "semgrep";
          repo = "semgrep-rules";
          rev = "d375208f04370b4e8d3ca7fe668db6f0465bb643";
          hash = "sha256-2fU2LZGEiR4W/CGPir9e41Elf9OfxK2tUcVYKocZVAI=";
        };
      in
        runCommand "check-scan" {
          nativeBuildInputs = with pkgs; [
            git
            semgrep
          ];
        } ''
          cd ${./.}
          mkdir -p "$TMP/scan"
          HOME="$TMP/scan"

          semgrep scan --quiet --error --metrics=off --config="${rules}/go"

          touch $out
        '';

      build = with pkgs;
        buildGoModule {
          pname = "check-build";
          inherit version;
          src = ./.;
          goSum = ./go.sum;
          vendorHash = "sha256-7/Z5A3ZXGT63GLjtWXKLiwHtp+ROGcxdqIzZhDgGH4w=";
          env.CGO_ENABLED = 0;

          preBuild = ''
            HOME=$PWD
            cp -r ${ts-web.packages."${system}".default} client
          '';

          installPhase = ''
            touch $out
          '';
        };

      db = with pkgs;
        runCommandLocal "check-db" {
          nativeBuildInputs = with pkgs; [
            sqlite
            dbmate
          ];
        } ''
          cd ${./.}
          HOME=$PWD

          export DATABASE_URL=sqlite:$TMP/check.db
          dbmate up

          touch $out
        '';
    });

    formatter = forSystem ({pkgs, ...}: pkgs.alejandra);

    packages = forSystem (
      {
        pkgs,
        system,
        ...
      }: let
        server = pkgs.buildGoModule {
          inherit pname version;
          src = ./.;
          goSum = ./go.sum;
          vendorHash = "sha256-7/Z5A3ZXGT63GLjtWXKLiwHtp+ROGcxdqIzZhDgGH4w=";
          env.CGO_ENABLED = 0;

          preBuild = ''
            HOME=$PWD
            cp -r ${ts-web.packages."${system}".default} client
          '';
        };

        binaries = builtins.listToAttrs (builtins.map (x: {
            name = "${pname}-${x.GOOS}-${x.GOARCH}";
            value = server.overrideAttrs {
              nativeBuildInputs =
                server.nativeBuildInputs
                ++ [
                  pkgs.rename
                ];
              env.CGO_ENABLED = 0;
              env.GOOS = x.GOOS;
              env.GOARCH = x.GOARCH;
              doCheck = false;

              installPhase = ''
                runHook preInstall

                mkdir -p $out/bin
                find $GOPATH/bin -type f -exec mv -t $out/bin {} +
                rename 's/(.+\/)(.+?)(\.[^.]*$|$)/$1${pname}-${x.GOOS}-${x.GOARCH}-${version}$3/' $out/bin/*

                runHook postInstall
              '';
            };
          })
          host-systems);

        images = builtins.listToAttrs (builtins.map (x: {
            name = "${pname}-${x.GOOS}-${x.GOARCH}-image";
            value = pkgs.dockerTools.buildImage {
              name = "${pname}";
              tag = "${version}-${x.GOARCH}";
              created = "now";
              architecture = "${x.GOARCH}";
              copyToRoot = [binaries."${pname}-${x.GOOS}-${x.GOARCH}"];
              config = {
                Cmd = ["${binaries."${pname}-${x.GOOS}-${x.GOARCH}"}/bin/${pname}-${x.GOOS}-${x.GOARCH}-${version}"];
              };
            };
          })
          (builtins.filter (x: x.GOOS == "linux") host-systems));
      in
        {
          default = server;
        }
        // binaries
        // images
    );
  };
}
