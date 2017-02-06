class Box < Formula
  desc "A Next-Generation Builder for Docker Images"
  homepage "https://erikh.github.io/box/"
  url "https://github.com/erikh/box/releases/download/v0.4.1/box-0.4.1.darwin.gz"
  sha256 "4e7241614b2f091ab3f1a19d0707314db060717ae555f2c47c8c8374c4621aad"

  def install
    mv "box-0.4.1.darwin", "box"
    bin.install "box"
  end
  test do
    system "#{bin}/box", "--version"
  end
end
