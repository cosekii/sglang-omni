// SPDX-License-Identifier: Apache-2.0
// Verification-only driver for the exact PR's MossRealtimeSession.
#include <chrono>
#include <condition_variable>
#include <cstdlib>
#include <iostream>
#include <memory>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

#include "common/audio.h"
#include "moss/moss_realtime.h"

namespace {

std::string ZeroAudioBase64(int sample_count) {
  const size_t byte_count = static_cast<size_t>(sample_count) * 2;
  std::string encoded;
  encoded.reserve((byte_count + 2) / 3 * 4);
  const size_t whole_triples = byte_count / 3 * 3;
  for (size_t i = 0; i < whole_triples; i += 3) encoded += "AAAA";
  if (byte_count - whole_triples == 1) {
    encoded += "AA==";
  } else if (byte_count - whole_triples == 2) {
    encoded += "AAA=";
  }
  return encoded;
}

struct Collector {
  omni_server::RealtimeConnection::Sender Sender() {
    return [this](const std::string &line) {
      {
        std::lock_guard<std::mutex> lock(mutex);
        events.push_back(nlohmann::json::parse(line));
      }
      cv.notify_all();
      return true;
    };
  }

  bool WaitForSegments(size_t count,
                       std::chrono::milliseconds timeout =
                           std::chrono::milliseconds(2000)) {
    std::unique_lock<std::mutex> lock(mutex);
    return cv.wait_for(lock, timeout, [&] { return SegmentCount() >= count; });
  }

  bool WaitForErrors(size_t count) {
    std::unique_lock<std::mutex> lock(mutex);
    return cv.wait_for(lock, std::chrono::seconds(2), [&] {
      size_t found = 0;
      for (const auto &event : events) {
        if (event.value("type", "") == "error") ++found;
      }
      return found >= count;
    });
  }

  size_t SegmentCount() const {
    size_t found = 0;
    for (const auto &event : events) {
      if (event.value("type", "") == "transcription.segment") ++found;
    }
    return found;
  }

  nlohmann::json Snapshot() const {
    std::lock_guard<std::mutex> lock(mutex);
    return events;
  }

  mutable std::mutex mutex;
  std::condition_variable cv;
  std::vector<nlohmann::json> events;
};

std::shared_ptr<moss::MossRealtimeSession> MakeSession(
    omni_server::TranscriptionWorker &worker,
    const moss::MossTranscriber &transcriber, Collector &collector) {
#ifdef OLD_SESSION_API
  auto session = std::make_shared<moss::MossRealtimeSession>(
      worker, transcriber, collector.Sender());
#else
  auto session = std::make_shared<moss::MossRealtimeSession>(
      worker,
      [&transcriber](const std::vector<float> &samples,
                     const moss::MossOptions &options,
                     const std::atomic<bool> &cancel) {
        return transcriber.Transcribe(samples, options, cancel);
      },
      collector.Sender());
#endif
  session->Handle({{"type", "session.update"},
                   {"session",
                    {{"turn_detection", nullptr},
                     {"prompt", "SYNTHETIC_PROMPT"}}}});
  return session;
}

void Append(const std::shared_ptr<moss::MossRealtimeSession> &session,
            int samples) {
  session->Handle({{"type", "input_audio_buffer.append"},
                   {"audio", ZeroAudioBase64(samples)}});
}

bool Done(const std::shared_ptr<moss::MossRealtimeSession> &session) {
  return !session->Handle({{"type", "transcription.done"}});
}

bool WaitEntered(int count) {
  std::unique_lock<std::mutex> lock(moss::g_transcribe_mutex);
  return moss::g_transcribe_cv.wait_for(lock, std::chrono::seconds(2), [&] {
    return moss::g_transcribe_entered >= count;
  });
}

int Fail(const char *message) {
  std::cerr << "BEHAVIOR_CHECK_FAILED " << message << "\n";
  return 1;
}

nlohmann::json Segments(const Collector &collector) {
  nlohmann::json result = nlohmann::json::array();
  for (const auto &event : collector.Snapshot()) {
    if (event.value("type", "") == "transcription.segment") {
      result.push_back(event);
    }
  }
  return result;
}

} // namespace

int main(int argc, char **argv) {
  if (argc < 2) return Fail("scenario argument required");
  const std::string scenario = argv[1];
  moss::MossTranscriber transcriber("", "", 0);
  omni_server::TranscriptionWorker worker([] {}, [] {}, 1);
  Collector collector;
  auto session = MakeSession(worker, transcriber, collector);

  if (scenario == "probe") {
    if (argc != 3) return Fail("probe sample count required");
    const int samples = std::stoi(argv[2]);
    Append(session, samples);
    collector.WaitForSegments(1, std::chrono::milliseconds(500));
    nlohmann::json evidence = {{"samples", samples},
                               {"transcribe_entered", moss::g_transcribe_entered},
                               {"segments", Segments(collector)}};
    std::cout << evidence.dump() << "\n";
    session->Close();
    return 0;
  }

  if (scenario == "cancel") {
    Append(session, 12000);
    if (!WaitEntered(1)) return Fail("cancel decode did not start");
    session->Close();
    return 0;
  }

  if (scenario == "short_stop") {
    const auto started = std::chrono::steady_clock::now();
    Append(session, 12000);
    std::this_thread::sleep_until(started + std::chrono::milliseconds(100));
    if (!Done(session)) return Fail("short-stop completion rejected");
    const auto elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
                             std::chrono::steady_clock::now() - started)
                             .count();
    nlohmann::json evidence = {{"elapsed_ms", elapsed},
                               {"segments", Segments(collector)},
                               {"events", collector.Snapshot()}};
    std::cout << evidence.dump() << "\n";
    session->Close();
    return 0;
  }

  if (scenario == "stale_short" || scenario == "stale_window") {
    moss::g_transcribe_gate.store(false);
    Append(session, 12000);
    if (!WaitEntered(1)) return Fail("barrier preview did not start");
    if (scenario == "stale_short") {
      bool done_ok = false;
      std::thread stopper([&] { done_ok = Done(session); });
      std::this_thread::sleep_for(std::chrono::milliseconds(100));
      moss::g_transcribe_gate.store(true);
      stopper.join();
      if (!done_ok) return Fail("stale-short completion rejected");
    } else {
      Append(session, 52000);
      moss::g_transcribe_gate.store(true);
      std::this_thread::sleep_for(std::chrono::milliseconds(600));
    }
    const auto segments = Segments(collector);
    std::cout << nlohmann::json{{"segments", segments},
                                {"events", collector.Snapshot()}}
              << "\n";
    session->Close();
    if (segments.size() != 1 || !segments[0].value("is_final", false)) {
      return Fail("a cancelled successful preview was published");
    }
    return 0;
  }

  if (scenario == "timeline_current" || scenario == "timeline_old") {
    const auto started = std::chrono::steady_clock::now();
    const auto at = [&](int milliseconds) {
      std::this_thread::sleep_until(started + std::chrono::milliseconds(milliseconds));
    };
    if (scenario == "timeline_current") {
      Append(session, 12000);
      at(1010);
      Append(session, 16000);
      at(2020);
      Append(session, 16000);
      at(3030);
      Append(session, 16000);
      at(3250);
      Append(session, 4000);
    } else {
      Append(session, 12000);
      at(500);
      Append(session, 8000);
      at(1510);
      Append(session, 16000);
      at(2520);
      Append(session, 16000);
      at(3250);
      Append(session, 12000);
    }
    if (!Done(session)) return Fail("timeline completion rejected");
    const auto elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
                             std::chrono::steady_clock::now() - started)
                             .count();
    nlohmann::json evidence = {{"elapsed_ms", elapsed},
                               {"segments", Segments(collector)},
                               {"events", collector.Snapshot()}};
    std::cout << evidence.dump() << "\n";
    session->Close();
    return 0;
  }

  if (scenario == "cadence") {
    Append(session, 12000);
    if (!collector.WaitForSegments(1)) return Fail("first preview missing");
    Append(session, 1);
    std::this_thread::sleep_for(std::chrono::milliseconds(100));
    if (collector.SegmentCount() != 1) return Fail("preview ignored 1 s cadence");
    std::this_thread::sleep_for(std::chrono::milliseconds(1050));
    Append(session, 1);
    if (!collector.WaitForSegments(2)) return Fail("second preview missing");
    for (const auto &segment : Segments(collector)) {
      if (segment.value("is_final", true)) return Fail("cadence emitted a final");
    }
  } else if (scenario == "final") {
    Append(session, 64000);
    if (!collector.WaitForSegments(1)) return Fail("full-window final missing");
    const auto segments = Segments(collector);
    if (segments.size() != 1 || !segments[0].value("is_final", false)) {
      return Fail("full-window final lost priority");
    }
    if (!Done(session)) return Fail("final completion rejected");
  } else if (scenario == "window_preempt") {
    Append(session, 12000);
    if (!WaitEntered(1)) return Fail("window preview did not start");
    Append(session, 52000);
    if (!collector.WaitForSegments(1)) {
      return Fail("filled window did not schedule its final");
    }
    const auto segments = Segments(collector);
    if (segments.size() != 1 || !segments[0].value("is_final", false)) {
      return Fail("cancelled preview published or full-window final missing");
    }
    if (!Done(session)) return Fail("window completion rejected");
  } else if (scenario == "flush") {
    Append(session, 30);
    std::this_thread::sleep_for(std::chrono::milliseconds(100));
    if (collector.SegmentCount() != 0) return Fail("short input previewed");
    if (!Done(session)) return Fail("short completion rejected");
    const auto segments = Segments(collector);
    if (segments.size() != 1 || !segments[0].value("is_final", false)) {
      return Fail("short input did not flush as final");
    }
  } else if (scenario == "invalid") {
    session->Handle(
        {{"type", "input_audio_buffer.append"}, {"audio", "not-base64"}});
    if (!Done(session)) return Fail("invalid completion rejected");
    bool invalid_audio = false;
    for (const auto &event : collector.Snapshot()) {
      invalid_audio |= event.value("error", nlohmann::json::object())
                           .value("code", "") == "invalid_audio";
    }
    if (!invalid_audio) return Fail("invalid audio error missing");
  } else if (scenario == "error") {
    Append(session, 12000);
    if (!collector.WaitForErrors(1)) return Fail("decode error missing");
    if (!Done(session)) return Fail("error completion rejected");
    const auto segments = Segments(collector);
    if (segments.size() != 1 || !segments[0].value("is_final", false)) {
      return Fail("session did not recover for final");
    }
  } else {
    return Fail("unknown scenario");
  }

  std::cout << collector.Snapshot().dump() << "\n";
  session->Close();
  return 0;
}
