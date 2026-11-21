import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ScValTree } from "./ScValTree";
import type { DecodedScVal } from "@sorolens/xdr";

describe("ScValTree", () => {
  const writeText = vi.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    writeText.mockClear();
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("renders a decoded symbol node with type badge and value", () => {
    const symbolVal: DecodedScVal = {
      type: "symbol",
      value: "transfer",
      human: "transfer",
    };

    render(
      <ScValTree label="topics[0]" decoded={symbolVal} path="topics[0]" />
    );

    expect(screen.getByText("topics[0]:")).toBeInTheDocument();
    expect(screen.getByText("symbol")).toBeInTheDocument();
    expect(screen.getByText("transfer")).toBeInTheDocument();
  });

  it("decodes valid XDR and displays the tree", () => {
    // "AAAADwAAAAhUcmFuc2Zlcg==" is Soroban ScVal symbol "Transfer"
    const xdr = "AAAADwAAAAhUcmFuc2Zlcg==";

    render(<ScValTree label="Topic" xdr={xdr} path="topics[0]" />);

    expect(screen.getByText("Topic:")).toBeInTheDocument();
    expect(screen.getByText("symbol")).toBeInTheDocument();
    expect(screen.getByText("Transfer")).toBeInTheDocument();
  });

  it("renders vector nodes with expandable children", () => {
    const vecVal: DecodedScVal = {
      type: "vec",
      value: [
        { type: "symbol", value: "item1", human: "item1" },
        { type: "u32", value: 42, human: "42" },
      ],
      human: "[item1, 42]",
    };

    render(
      <ScValTree
        label="items"
        decoded={vecVal}
        path="value.items"
        defaultExpanded={true}
      />
    );

    expect(screen.getByText("items:")).toBeInTheDocument();
    expect(screen.getByText("vec")).toBeInTheDocument();
    expect(screen.getByText("[0]:")).toBeInTheDocument();
    expect(screen.getByText("[1]:")).toBeInTheDocument();
    expect(screen.getByText("item1")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();

    // Click vector header to collapse
    const containerHeader = screen.getByText("items:").parentElement!;
    fireEvent.click(containerHeader);

    // After collapse, children are hidden
    expect(screen.queryByText("[0]:")).not.toBeInTheDocument();
  });

  it("renders map entries with keys and values", () => {
    const mapVal: DecodedScVal = {
      type: "map",
      value: [
        {
          key: { type: "symbol", value: "sender", human: "sender" },
          value: { type: "address", value: "GB...", human: "GB..." },
        },
      ],
      human: "{sender: GB...}",
    };

    render(
      <ScValTree
        label="payload"
        decoded={mapVal}
        path="value"
        defaultExpanded={true}
      />
    );

    expect(screen.getByText("payload:")).toBeInTheDocument();
    expect(screen.getByText("map")).toBeInTheDocument();
    expect(screen.getByText("key:")).toBeInTheDocument();
    expect(screen.getByText("sender")).toBeInTheDocument();
    expect(screen.getByText("value:")).toBeInTheDocument();
    expect(screen.getByText("GB...")).toBeInTheDocument();
  });

  it("copies path and value to clipboard", async () => {
    const primitiveVal: DecodedScVal = {
      type: "u64",
      value: 123456789,
      human: "123456789",
    };

    render(
      <ScValTree label="amount" decoded={primitiveVal} path="value.amount" />
    );

    const copyPathBtn = screen.getByTitle("Copy path: value.amount");
    fireEvent.click(copyPathBtn);
    await waitFor(() => expect(writeText).toHaveBeenCalledWith("value.amount"));

    writeText.mockClear();
    const copyValBtn = screen.getByTitle("Copy value");
    fireEvent.click(copyValBtn);
    await waitFor(() => expect(writeText).toHaveBeenCalledWith("123456789"));
  });

  it("handles decode errors gracefully with error banner and raw copy", async () => {
    render(
      <ScValTree
        label="corrupt"
        xdr="THIS_IS_NOT_VALID_XDR_DATA!!!"
        path="topics[0]"
      />
    );

    expect(screen.getByText("corrupt:")).toBeInTheDocument();
    expect(screen.getByText("Decode Error")).toBeInTheDocument();
    expect(
      screen.getByText(/Failed to decode ScVal XDR payload/i)
    ).toBeInTheDocument();

    const copyRawBtn = screen.getByRole("button", { name: "Copy Raw XDR" });
    fireEvent.click(copyRawBtn);
    await waitFor(() =>
      expect(writeText).toHaveBeenCalledWith("THIS_IS_NOT_VALID_XDR_DATA!!!")
    );
  });
});
