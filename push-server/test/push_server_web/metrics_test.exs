defmodule PushServerWeb.MetricsTest do
  use ExUnit.Case, async: true

  test "Endpoint module exists" do
    assert Code.ensure_loaded?(PushServerWeb.Endpoint)
  end
end
