import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, render, screen, fireEvent } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TagInput } from "./TagInput";

expect.extend(matchers);

describe("TagInput", () => {
  afterEach(cleanup);

  it("adds a normalized tag on Enter", () => {
    const onAdd = vi.fn();
    render(<TagInput tags={[]} onAdd={onAdd} onRemove={vi.fn()} />);

    const input = screen.getByTestId("tag-input");
    fireEvent.change(input, { target: { value: "  Prod  " } });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onAdd).toHaveBeenCalledWith("prod");
    expect((input as HTMLInputElement).value).toBe("");
  });

  it("adds a tag on comma", () => {
    const onAdd = vi.fn();
    render(<TagInput tags={[]} onAdd={onAdd} onRemove={vi.fn()} />);

    const input = screen.getByTestId("tag-input");
    fireEvent.change(input, { target: { value: "defi" } });
    fireEvent.keyDown(input, { key: "," });

    expect(onAdd).toHaveBeenCalledWith("defi");
  });

  it("rejects an invalid tag and shows an error", () => {
    const onAdd = vi.fn();
    render(<TagInput tags={[]} onAdd={onAdd} onRemove={vi.fn()} />);

    const input = screen.getByTestId("tag-input");
    fireEvent.change(input, { target: { value: "bad tag" } });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onAdd).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toBeDefined();
  });

  it("does not re-add a tag that is already present", () => {
    const onAdd = vi.fn();
    render(<TagInput tags={["prod"]} onAdd={onAdd} onRemove={vi.fn()} />);

    const input = screen.getByTestId("tag-input");
    fireEvent.change(input, { target: { value: "prod" } });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onAdd).not.toHaveBeenCalled();
  });

  it("removes a tag via its chip button", () => {
    const onRemove = vi.fn();
    render(
      <TagInput tags={["prod", "defi"]} onAdd={vi.fn()} onRemove={onRemove} />
    );

    fireEvent.click(screen.getByLabelText("Remove tag prod"));

    expect(onRemove).toHaveBeenCalledWith("prod");
  });

  it("removes the last tag on Backspace when the input is empty", () => {
    const onRemove = vi.fn();
    render(
      <TagInput tags={["prod", "defi"]} onAdd={vi.fn()} onRemove={onRemove} />
    );

    fireEvent.keyDown(screen.getByTestId("tag-input"), { key: "Backspace" });

    expect(onRemove).toHaveBeenCalledWith("defi");
  });

  it("surfaces a server error passed by the parent", () => {
    render(
      <TagInput
        tags={[]}
        onAdd={vi.fn()}
        onRemove={vi.fn()}
        error="You need a contributor identity to edit tags."
      />
    );
    expect(screen.getByRole("alert")).toBeDefined();
  });
});
