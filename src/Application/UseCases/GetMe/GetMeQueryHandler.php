<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Application\UseCases\GetMe;

use Symfony\Component\Messenger\Attribute\AsMessageHandler;

#[AsMessageHandler(bus: 'query.bus')]
readonly class GetMeQueryHandler
{
    public function __invoke(GetMeQuery $getMeQuery): GetMeResult
    {
        return new GetMeResult(
            $getMeQuery->id,
            $getMeQuery->email,
            $getMeQuery->phone,
            $getMeQuery->firstName,
            $getMeQuery->lastName,
        );
    }

}
