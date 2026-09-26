from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.verification_source_input_kind import VerificationSourceInputKind
from ..types import UNSET, Unset

T = TypeVar("T", bound="VerificationSourceInput")


@_attrs_define
class VerificationSourceInput:
    """
    Attributes:
        kind (VerificationSourceInputKind): `archive` requires `content_base64`; `git` requires `url` and
            `commit`.
        filename (str | Unset): Archive name, used in the build log. Archive sources only.
        content_base64 (str | Unset): Standard-base64 encoded `.tar.gz` of the contract source. Archive
            sources only. The decoded payload is capped at 8 MiB.
        url (str | Unset): HTTPS Git URL to clone. Git sources only.
        commit (str | Unset): Commit, tag, or branch to check out. Git sources only.
        subdir (str | Unset): Crate directory inside the materialized source, for archives or
            repositories holding more than one crate.
    """

    kind: VerificationSourceInputKind
    filename: str | Unset = UNSET
    content_base64: str | Unset = UNSET
    url: str | Unset = UNSET
    commit: str | Unset = UNSET
    subdir: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        filename = self.filename

        content_base64 = self.content_base64

        url = self.url

        commit = self.commit

        subdir = self.subdir

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "kind": kind,
            }
        )
        if filename is not UNSET:
            field_dict["filename"] = filename
        if content_base64 is not UNSET:
            field_dict["content_base64"] = content_base64
        if url is not UNSET:
            field_dict["url"] = url
        if commit is not UNSET:
            field_dict["commit"] = commit
        if subdir is not UNSET:
            field_dict["subdir"] = subdir

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        kind = VerificationSourceInputKind(d.pop("kind"))

        filename = d.pop("filename", UNSET)

        content_base64 = d.pop("content_base64", UNSET)

        url = d.pop("url", UNSET)

        commit = d.pop("commit", UNSET)

        subdir = d.pop("subdir", UNSET)

        verification_source_input = cls(
            kind=kind,
            filename=filename,
            content_base64=content_base64,
            url=url,
            commit=commit,
            subdir=subdir,
        )

        verification_source_input.additional_properties = d
        return verification_source_input

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
