// SPDX-License-Identifier: Apache-2.0
#include <iostream>
#include <stdexcept>
#include <string>

#include "moss_model.cpp"

namespace {

namespace mx = mlx::core;

void Require(bool condition, const std::string &message) {
  if (!condition) {
    throw std::runtime_error(message);
  } else {
  }
}

void RequireEqual(const mx::array &actual, const mx::array &expected,
                  const std::string &message) {
  Require(mx::all(mx::array_equal(actual, expected)).item<bool>(), message);
}

} // namespace

int main() {
  const std::string decoder = "model.language_model";
  const std::string embedding = decoder + ".embed_tokens.weight";
  const std::string projection = decoder + ".output_projection";
  const std::string projection_weight = projection + ".weight";
  const auto embedding_values = mx::ones({2, 64}, mx::float32);
  const auto projection_values = mx::add(embedding_values, embedding_values);
  const auto quantized = mx::quantize(projection_values, 64, 8);
  mx::async_eval(quantized);

  omni_common::WeightMap prequantized;
  prequantized.emplace(embedding, embedding_values);
  prequantized.emplace(projection_weight, quantized[0]);
  prequantized.emplace(projection + ".scales", quantized[1]);
  prequantized.emplace(projection + ".biases", quantized[2]);
  prequantized.emplace("lm_head.weight", mx::zeros({2, 64}, mx::float32));
  prequantized.emplace("unrelated.weight", mx::full({1, 1}, 42.0f));
  prequantized.emplace("model.whisper_encoder.conv1.weight",
                       mx::ones({2, 3, 4}, mx::float32));

  auto loaded = moss::LoadedWeights(prequantized);
  const nlohmann::json prequantized_config = {
      {"quantization", {{"group_size", 64}, {"bits", 8}}}};
  auto prequantized_checkpoint = omni_common::QuantizedCheckpoint(
      loaded, prequantized_config, 64,
      [](const std::string &) { return 0; });
  const auto input = mx::ones({1, 64}, mx::float32);
  const auto prequantized_output =
      prequantized_checkpoint.Linear(input, projection);
  RequireEqual(prequantized_output,
               mx::matmul(input, mx::transpose(projection_values)),
               "prequantized Linear output differs from the source projection");
  Require(loaded.at(projection_weight).dtype() == mx::uint32,
          "prequantized projection dtype was replaced");
  RequireEqual(loaded.at(projection_weight), quantized[0],
               "prequantized projection values were replaced");
  RequireEqual(loaded.at(projection + ".scales"), quantized[1],
               "quantization scales changed");
  RequireEqual(loaded.at(projection + ".biases"), quantized[2],
               "quantization biases changed");
  RequireEqual(loaded.at("unrelated.weight"), mx::full({1, 1}, 42.0f),
               "unrelated weight changed");
  Require(!loaded.contains("lm_head.weight"), "lm_head was not removed");
  Require(loaded.at("model.whisper_encoder.conv1.weight").shape() ==
              mx::Shape{2, 4, 3},
          "Conv1d layout did not retain its existing transpose");

  omni_common::WeightMap legacy;
  legacy.emplace(embedding, embedding_values);
  auto legacy_loaded = moss::LoadedWeights(legacy);
  Require(legacy_loaded.at(projection_weight).dtype() == mx::float32,
          "missing projection did not retain the tied embedding dtype");
  RequireEqual(legacy_loaded.at(projection_weight), embedding_values,
               "missing projection was not tied to the embedding");
  auto legacy_checkpoint = omni_common::QuantizedCheckpoint(
      legacy_loaded, nlohmann::json::object(), 64,
      [&](const std::string &name) {
        return name == projection_weight ? 8 : 0;
      });
  Require(legacy_checkpoint.Weight(projection_weight).dtype() == mx::uint32,
          "legacy projection was not quantized at load");
  RequireEqual(legacy_checkpoint.Linear(input, projection),
               mx::matmul(input, mx::transpose(embedding_values)),
               "legacy quantized Linear output differs from the embedding");

  std::cout << "prequantized projection and quantized_matmul: PASS\n"
               "missing projection fallback and load-time quantization: PASS\n"
               "metadata, unrelated weight, lm_head, and Conv1d layout: PASS\n";
  return 0;
}
