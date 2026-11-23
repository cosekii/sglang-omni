// Extra review probe: include the checked-in fixtures without running its main.
#define main checked_in_preference_test_main
#include "tokenizer_preference_test.cpp"
#undef main

namespace {

void TestMergeRanksAreReset() {
  const std::filesystem::path directory = FreshDirectory("poison-merges");
  const nlohmann::json vocab = {{"a", 0},  {"b", 1}, {"c", 2},
                                {"ab", 3}, {"bc", 4}, {"abc", 5}};
  std::ofstream(directory / "vocab.json") << vocab.dump();
  std::ofstream(directory / "merges.txt")
      << "#version: 0.2\na b\nab c\nb c\n";
  std::ofstream(directory / "tokenizer_config.json")
      << R"({"added_tokens_decoder": {}})";

  nlohmann::json poisoned;
  poisoned["model"] = {{"type", "BPE"}, {"vocab", vocab}};
  poisoned["model"]["merges"] = nlohmann::json::array(
      {nlohmann::json::array({"b", "c"}),
       nlohmann::json::array({"a", "b"}),
       nlohmann::json::array({"ab"})});
  poisoned["normalizer"] = {{"type", "NFC"}};
  poisoned["pre_tokenizer"] =
      SplitPreTokenizer(omni_common::kQwen2SplitPattern);
  poisoned["added_tokens"] = nlohmann::json::array();
  std::ofstream(directory / "tokenizer.json") << poisoned.dump(2);

  const omni_common::Tokenizer fallback(directory);
  Require(fallback.Encode("abc") == std::vector<int>{5},
          "merge ranks from failed JSON must not reorder legacy BPE merges");
}

void TestAddedTokenIndexIsReset() {
  const std::filesystem::path directory = FreshDirectory("poison-added-index");
  WriteLegacyFiles(directory);
  nlohmann::json poisoned;
  poisoned["model"] = {{"type", "BPE"},
                       {"vocab", {{"h", 50}}},
                       {"merges", nlohmann::json::array()}};
  poisoned["normalizer"] = {{"type", "NFC"}};
  poisoned["pre_tokenizer"] =
      SplitPreTokenizer(omni_common::kQwen2SplitPattern);
  nlohmann::json bad = AddedToken(43, "<|y|>", false);
  bad["normalized"] = true;
  poisoned["added_tokens"] = {AddedToken(42, "<|x|>", false), bad};
  std::ofstream(directory / "tokenizer.json") << poisoned.dump(2);

  const omni_common::Tokenizer fallback(directory);
  Require(fallback.Decode({42}, false).empty(),
          "an added-token id from failed JSON must not decode as a legacy token");
  Require(fallback.Decode({99}, false) == "<|endoftext|>",
          "the legacy added-token index must survive fallback");
}

} // namespace

int main() {
  TestMergeRanksAreReset();
  TestAddedTokenIndexIsReset();
  std::filesystem::remove_all(kRoot);
  return 0;
}
