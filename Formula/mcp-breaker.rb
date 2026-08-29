class McpBreaker < Formula
  desc "Semantic MCP circuit breaker proxy for AI agent tool loops"
  homepage "https://github.com/shunvel/mcp-breaker"
  url "https://github.com/shunvel/mcp-breaker/archive/refs/tags/v0.2.0.tar.gz"
  sha256 "SKIP" # update after v0.2.0 tag is published
  license "Apache-2.0"

  depends_on "go" => :build
  depends_on "onnxruntime"

  def install
    system "go", "build", "-o", bin/"mcp-breaker", "./cmd/mcp-breaker"
  end

  def caveats
    onnx = Formula["onnxruntime"].opt_lib/"libonnxruntime.dylib"
    <<~EOS
      Semantic embeddings require the ONNX model cache:
        mcp-breaker models download

      If ONNX Runtime is not found automatically, set:
        export ONNXRUNTIME_LIB_PATH=#{onnx}
    EOS
  end

  test do
    assert_match "mcp-breaker", shell_output("#{bin}/mcp-breaker help", 2)
  end
end
