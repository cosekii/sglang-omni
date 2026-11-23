#include <filesystem>
#include <iostream>
#include <stdexcept>
#include <string>
#include <vector>

#include "../src/common/tokenizer.cpp"

int main(int argc, char **argv) {
  if (argc != 2) {
    throw std::invalid_argument("usage: real_directory_test MODEL_DIRECTORY");
  }
  const omni_common::Tokenizer tokenizer{
      std::filesystem::path(argv[1])};
  const std::vector<int> ids = tokenizer.Encode("Hello, world!");
  if (ids.empty() || tokenizer.Decode(ids, true) != "Hello, world!") {
    throw std::runtime_error("real tokenizer did not round-trip ordinary text");
  }
  const int end = tokenizer.AddedTokenId("<|endoftext|>");
  if (end != 50257) {
    throw std::runtime_error("Whisper endoftext id was not loaded from legacy config");
  }
  const std::string with_special = "Hello<|endoftext|>";
  if (tokenizer.Decode(tokenizer.Encode(with_special), false) != with_special) {
    throw std::runtime_error("real tokenizer did not round-trip an added token");
  }
  std::cout << "ordinary_ids=" << ids.size() << " endoftext_id=" << end << '\n';
  return 0;
}
