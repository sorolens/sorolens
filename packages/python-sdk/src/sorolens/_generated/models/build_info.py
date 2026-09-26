from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="BuildInfo")


@_attrs_define
class BuildInfo:
    """
    Attributes:
        version (str): Semantic version injected at build time; "dev" when unset. Example: 1.4.2.
        git_sha (str): Git commit SHA injected at build time; "dev" when unset. Example: abc1234.
        built_at (str): RFC3339 build timestamp injected at build time; "dev" when unset or malformed. Example:
            2026-01-02T15:04:05Z.
    """

    version: str
    git_sha: str
    built_at: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        version = self.version

        git_sha = self.git_sha

        built_at = self.built_at

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "version": version,
                "git_sha": git_sha,
                "built_at": built_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        version = d.pop("version")

        git_sha = d.pop("git_sha")

        built_at = d.pop("built_at")

        build_info = cls(
            version=version,
            git_sha=git_sha,
            built_at=built_at,
        )

        build_info.additional_properties = d
        return build_info

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
