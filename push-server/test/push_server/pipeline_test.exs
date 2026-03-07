defmodule PushServer.PipelineTest do
  use ExUnit.Case, async: true

  alias PushServer.Pipeline

  test "Pipeline module exists" do
    assert Code.ensure_loaded?(Pipeline)
  end

  test "handle_message/3 is exported" do
    assert function_exported?(Pipeline, :handle_message, 3)
  end

  test "handle_failed/2 is exported" do
    assert function_exported?(Pipeline, :handle_failed, 2)
  end

  test "handle_failed/2 returns messages without crashing" do
    failed_msg = %Broadway.Message{
      data: "bad json",
      acknowledger: {Broadway.NoopAcknowledger, nil, nil},
      status: {:failed, "decode_error"},
      metadata: %{}
    }

    result = Pipeline.handle_failed([failed_msg], %{})
    assert is_list(result)
    assert length(result) == 1
  end
end
