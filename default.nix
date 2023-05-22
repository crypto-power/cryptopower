{ stdenv, fetchFromGitHub, go }: 

let
  # Specify the version and repository information of your Go project
  version = "0.0.1";
  repoUrl = "https://github.com/planetdecred/godcr.git";
in stdenv.mkDerivation rec {
  name = "godcr-${version}";
  src = fetchFromGitHub {
    owner = "planetdecred";
    repo = "godcr";
    rev = version;
    sha256 = "7a98943822f442f0f493b1ddbc383446e552526d";
  };

 # Specify the dependencies of your project
  buildInputs = [ go ];

  # Build command for your project
  buildPhase = ''
    go build -o godcr .
  '';

  # Install command for your project
  installPhase = ''
    mkdir -p $out/bin
    cp godcr $out/bin/
  '';
}
