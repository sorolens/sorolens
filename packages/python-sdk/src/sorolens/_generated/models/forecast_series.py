from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.forecast_series_metric import ForecastSeriesMetric
from typing import cast

if TYPE_CHECKING:
  from ..models.forecast_point import ForecastPoint





T = TypeVar("T", bound="ForecastSeries")



@_attrs_define
class ForecastSeries:
    """ 
        Attributes:
            metric (ForecastSeriesMetric):
            horizon (int):
            daily_count (int):
            points (list[ForecastPoint]):
     """

    metric: ForecastSeriesMetric
    horizon: int
    daily_count: int
    points: list[ForecastPoint]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.forecast_point import ForecastPoint # noqa: PLC0415
        metric = self.metric.value

        horizon = self.horizon

        daily_count = self.daily_count

        points = []
        for points_item_data in self.points:
            points_item = points_item_data.to_dict()
            points.append(points_item)




        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "metric": metric,
            "horizon": horizon,
            "daily_count": daily_count,
            "points": points,
        })

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.forecast_point import ForecastPoint # noqa: PLC0415
        d = dict(src_dict)
        metric = ForecastSeriesMetric(d.pop("metric"))




        horizon = d.pop("horizon")

        daily_count = d.pop("daily_count")

        points = []
        _points = d.pop("points")
        for points_item_data in (_points):
            points_item = ForecastPoint.from_dict(points_item_data)



            points.append(points_item)


        forecast_series = cls(
            metric=metric,
            horizon=horizon,
            daily_count=daily_count,
            points=points,
        )


        forecast_series.additional_properties = d
        return forecast_series

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
